# Bakes the light of a level made in Blender into a lightmap for GoLib, and
# exports the level with it. Run it with Blender in the background, on the
# level's .blend, from the project root:
#
#   blender --background level.blend --python tools/blender/bake_lightmaps.py -- \
#       --output games/<game>/assets/models/level.glb
#
# (/Applications/Blender.app/Contents/MacOS/Blender on macOS.) It writes the
# .glb and, beside it, <name>.lightmap.hdr, and leaves the .blend as it was
# unless --save is given.
#
# What it bakes, with Cycles, for every still object (every visible mesh but
# those whose custom property static is false, the things that move in the
# game): the light of the sky, as much of it as reaches each place, of glowing
# materials and of area lamps, and the light the sun, the point lamps and
# the spot lamps bounce off everything around. The sun and those lamps light
# the level directly in GoLib, every frame, with their shadows: Model.Sun and
# Model.Lights give them, exported in Blender's own units (Raw), so the two
# halves match. See docs/ai/making-a-game.md and framework/README.md.
#
# It also bakes light probes, for what moves through the level: a grid of
# points over it, but those inside walls, each holding the light that comes
# from every way, in the same two halves, as four spherical harmonics each.
# They go in <name>.probes.json beside the .glb, which an empty named
# "GoLib light probes" names in its custom property lightprobes.
#
# And reflection probes: every empty whose custom property reflection is
# true, shown as a cube and scaled over a room, gets a panorama rendered by
# Cycles from its middle, <name>.reflection<n>.hdr, which shiny things in its
# box reflect, the box's walls where they are.
#
# Every still object gets a second UV map, Lightmap, unwrapped and packed for
# all of them together, as dense on a big wall as on a small crate, unless
# each has one already: then it is used as it is, so a hand-made one stays.
# Each object's custom property lightmap names the file, which GoLib reads.

import argparse
import json
import math
import os
import subprocess
import sys
import tempfile
import time

import bmesh
import bpy
import numpy as np
from mathutils import Matrix, Vector

LIGHTMAP_UV = "Lightmap"
REALTIME_LAMPS = {"SUN", "POINT", "SPOT"}


def parse_arguments():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    parser = argparse.ArgumentParser(prog="bake_lightmaps.py", description="Bake a Blender level's light into a lightmap for GoLib.")
    parser.add_argument("--output", required=True, help="the .glb to write; the lightmap goes beside it")
    parser.add_argument("--size", type=int, default=1024, help="the lightmap's width and height in texels (default 1024)")
    parser.add_argument("--samples", type=int, default=256, help="Cycles samples a texel: more is smoother and slower (default 256)")
    parser.add_argument("--margin", type=int, default=4, help="texels between charts, filled so they don't bleed (default 4)")
    parser.add_argument("--gpu", action="store_true", help="bake on the graphics card, when Blender can")
    parser.add_argument("--no-denoise", action="store_true", help="keep Cycles' grain instead of smoothing it with Intel's Open Image Denoise")
    parser.add_argument("--save", action="store_true", help="save the .blend with its Lightmap UV maps")
    parser.add_argument("--probe-spacing", type=float, default=2.0, help="meters between light probes (default 2)")
    parser.add_argument("--no-probes", action="store_true", help="bake no light probes")
    parser.add_argument("--reflection-size", type=int, default=512, help="the width of a reflection probe's panorama, twice its height (default 512)")
    return parser.parse_args(argv)


def fail(message):
    print("[fail] " + message)
    sys.exit(1)


def still_objects(scene):
    """The meshes to bake: visible, rendered, and not marked static = false."""
    found = []
    for o in scene.objects:
        if o.type != 'MESH' or o.hide_render or not o.visible_get():
            continue
        if "static" in o.keys() and not o["static"]:
            continue
        if len(o.data.polygons) == 0:
            continue
        found.append(o)
    return found


def uv_area(o):
    uv = o.data.uv_layers[LIGHTMAP_UV].data
    total = 0.0
    for p in o.data.polygons:
        points = [uv[i].uv for i in p.loop_indices]
        twice = 0.0
        for k in range(len(points)):
            x1, y1 = points[k]
            x2, y2 = points[(k + 1) % len(points)]
            twice += x1 * y2 - x2 * y1
        total += abs(twice) / 2
    return total


def world_area(o):
    m = o.matrix_world
    vertices = o.data.vertices
    total = 0.0
    for p in o.data.polygons:
        points = [m @ vertices[i].co for i in p.vertices]
        for k in range(1, len(points) - 1):
            total += (points[k] - points[0]).cross(points[k + 1] - points[0]).length / 2
    return total


def unwrap(objects, margin):
    """Gives every object a Lightmap UV map, second after its own, packed for
    all of them at one density, unless all have one already."""
    if all(LIGHTMAP_UV in o.data.uv_layers for o in objects):
        print("[info] every object has a Lightmap UV map already: using them as they are")
        return
    for o in objects:
        layers = o.data.uv_layers
        if LIGHTMAP_UV in layers:
            layers.remove(layers[LIGHTMAP_UV])
        if len(layers) == 0:
            layers.new(name="UVMap")  # its textures' coordinates come first
        if len(layers) > 1:
            fail(f"{o.name} has {len(layers)} UV maps: GoLib reads the first for textures and the second, named {LIGHTMAP_UV}, for the lightmap. Remove the others, or name the second {LIGHTMAP_UV}")
        layers.active = layers.new(name=LIGHTMAP_UV)
        layers[0].active_render = True
    bpy.ops.object.select_all(action='DESELECT')
    for o in objects:
        o.select_set(True)
    bpy.context.view_layer.objects.active = objects[0]
    bpy.ops.object.mode_set(mode='EDIT')
    bpy.ops.mesh.select_all(action='SELECT')
    bpy.ops.uv.smart_project(angle_limit=math.radians(66), island_margin=0.0, scale_to_bounds=False)
    bpy.ops.object.mode_set(mode='OBJECT')
    # Blender unwraps each mesh in its own space: sized by its area in the
    # world, a big wall gets as many texels a meter as a small crate.
    for o in objects:
        area = uv_area(o)
        if area <= 0:
            continue
        k = math.sqrt(world_area(o) / area)
        for d in o.data.uv_layers[LIGHTMAP_UV].data:
            d.uv = d.uv * k
    bpy.ops.object.mode_set(mode='EDIT')
    bpy.ops.mesh.select_all(action='SELECT')
    bpy.ops.uv.select_all(action='SELECT')
    bpy.ops.uv.pack_islands(margin=margin, rotate=True, scale=True)
    bpy.ops.object.mode_set(mode='OBJECT')


class Bake:
    """The image nodes and materials added for the bake, taken out after."""

    def __init__(self, objects, image):
        self.added_nodes = []
        self.added_materials = []
        for o in objects:
            if len(o.data.materials) == 0:
                m = bpy.data.materials.new("GoLib bake")
                m.use_nodes = True
                o.data.materials.append(m)
                self.added_materials.append((o, m))
            for m in o.data.materials:
                if m is None or not m.use_nodes or any(n for n in self.added_nodes if n.id_data == m.node_tree):
                    continue
                tree = m.node_tree
                uv = tree.nodes.new("ShaderNodeUVMap")
                uv.uv_map = LIGHTMAP_UV
                node = tree.nodes.new("ShaderNodeTexImage")
                node.image = image
                tree.links.new(uv.outputs["UV"], node.inputs["Vector"])
                tree.nodes.active = node
                self.added_nodes += [uv, node]

    def point_at(self, image):
        for n in self.added_nodes:
            if n.bl_idname == "ShaderNodeTexImage":
                n.image = image
                n.id_data.nodes.active = n

    def remove(self):
        for n in self.added_nodes:
            n.id_data.nodes.remove(n)
        for o, m in self.added_materials:
            o.data.materials.pop(index=list(o.data.materials).index(m))
            bpy.data.materials.remove(m)


def emissions():
    """The strength inputs of every glowing material, to turn them off."""
    found = []
    for m in bpy.data.materials:
        if not m.use_nodes:
            continue
        for n in m.node_tree.nodes:
            if n.bl_idname == "ShaderNodeBsdfPrincipled":
                found.append(n.inputs["Emission Strength"])
            elif n.bl_idname == "ShaderNodeEmission":
                found.append(n.inputs["Strength"])
    return found


def two_halves(scene, bake):
    """Calls bake(direct, indirect) twice and adds what it returns: the sky,
    glowing materials and area lamps, all their light; then the sun and the
    point and spot lamps, only what they bounce, since GoLib lights with them
    directly."""
    lamps = [o for o in scene.objects if o.type == 'LIGHT' and not o.hide_render]
    realtime = [o for o in lamps if o.data.type in REALTIME_LAMPS]
    baked_lamps = [o for o in lamps if o.data.type not in REALTIME_LAMPS]
    world = scene.world
    for o in realtime:
        o.hide_render = True
    try:
        around = bake(direct=True, indirect=True)
    finally:
        for o in realtime:
            o.hide_render = False
    if not realtime:
        return around
    for o in baked_lamps:
        o.hide_render = True
    scene.world = None
    glow = [(s, s.default_value) for s in emissions()]
    for s, _ in glow:
        s.default_value = 0.0
    try:
        bounced = bake(direct=False, indirect=True)
    finally:
        for s, value in glow:
            s.default_value = value
        scene.world = world
        for o in baked_lamps:
            o.hide_render = False
    return around + bounced


def to_golib(v):
    """A point or direction of Blender's, Z up, in GoLib's space, Y up, as
    the glTF exporter turns it."""
    return Vector((v.x, v.z, -v.y))


def to_blender(v):
    return Vector((v.x, -v.z, v.y))


def probe_grid(objects, spacing, limit=4096):
    """The grid of probe points over the still objects, in GoLib's space: its
    first corner, the meters between points, and how many along X, Y and Z,
    spread wider when there would be more than limit."""
    lo = Vector((math.inf,) * 3)
    hi = Vector((-math.inf,) * 3)
    for o in objects:
        for corner in o.bound_box:
            p = to_golib(o.matrix_world @ Vector(corner))
            lo = Vector(map(min, lo, p))
            hi = Vector(map(max, hi, p))
    # A point in the middle of every cell spacing meters wide, so none sits
    # on the floor or in the outer walls.
    while True:
        counts = [max(1, round((hi[i] - lo[i]) / spacing)) for i in range(3)]
        if counts[0] * counts[1] * counts[2] <= limit:
            break
        spacing *= 1.25
    origin = Vector([lo[i] + (hi[i] - lo[i] - (counts[i] - 1) * spacing) / 2 for i in range(3)])
    return origin, spacing, counts


def inside_something(scene, point, reach):
    """Whether point is inside a wall or a block: most ways out of it meet
    the back of a face."""
    depsgraph = bpy.context.evaluated_depsgraph_get()
    backs = 0
    for d in (Vector((1, 0, 0)), Vector((-1, 0, 0)), Vector((0, 1, 0)), Vector((0, -1, 0)), Vector((0, 0, 1)), Vector((0, 0, -1))):
        hit, _, normal, _, _, _ = scene.ray_cast(depsgraph, point, d, distance=reach)
        if hit and normal.dot(d) > 0:
            backs += 1
    return backs >= 3


def bake_probes(scene, objects, spacing):
    """Bakes the light probes, returning the grid and, for each point, its
    four spherical harmonics, red, green and blue each, or None inside a
    wall."""
    origin, spacing, counts = probe_grid(objects, spacing)
    points = []
    for z in range(counts[2]):
        for y in range(counts[1]):
            for x in range(counts[0]):
                p = origin + Vector((x, y, z)) * spacing
                at = to_blender(p)
                points.append(None if inside_something(scene, at, spacing * 4) else at)
    found = [p for p in points if p is not None]
    if not found:
        return origin, spacing, counts, [None] * len(points)

    # A small ball at every point, one mesh for all, that nothing else sees:
    # Cycles bakes the light reaching each of its vertices, from its way.
    bm = bmesh.new()
    starts = []
    for p in found:
        starts.append(len(bm.verts))
        bmesh.ops.create_icosphere(bm, subdivisions=2, radius=0.02, matrix=Matrix.Translation(p))
    per = len(bm.verts) // len(found)
    mesh = bpy.data.meshes.new("GoLib probes")
    bm.to_mesh(mesh)
    bm.free()
    mesh.color_attributes.new("light", 'FLOAT_COLOR', 'POINT')
    mesh.color_attributes.active_color = mesh.color_attributes["light"]
    material = bpy.data.materials.new("GoLib probes")
    material.use_nodes = True
    mesh.materials.append(material)
    balls = bpy.data.objects.new("GoLib probes", mesh)
    scene.collection.objects.link(balls)
    # Hidden from every ray but the camera's: Cycles leaves out an object no
    # ray sees, and would bake nothing for it.
    for flag in ("visible_diffuse", "visible_glossy", "visible_transmission", "visible_volume_scatter", "visible_shadow"):
        setattr(balls, flag, False)

    def bake(direct, indirect):
        scene.render.bake.use_pass_direct = direct
        scene.render.bake.use_pass_indirect = indirect
        scene.render.bake.use_pass_color = False
        bpy.ops.object.select_all(action='DESELECT')
        balls.select_set(True)
        bpy.context.view_layer.objects.active = balls
        bpy.ops.object.bake(type='DIFFUSE', target='VERTEX_COLORS')
        colors = np.empty(len(mesh.vertices) * 4, dtype=np.float32)
        mesh.color_attributes["light"].data.foreach_get("color", colors)
        return colors

    try:
        light = two_halves(scene, bake).reshape(-1, 4)[:, :3]
        positions = np.empty(len(mesh.vertices) * 3, dtype=np.float32)
        mesh.vertices.foreach_get("co", positions)
        positions = positions.reshape(-1, 3)
    finally:
        bpy.data.objects.remove(balls)
        bpy.data.meshes.remove(mesh)
        bpy.data.materials.remove(material)

    # Each ball's light, fitted with the four spherical harmonics GoLib
    # reads: the constant, then Y, Z and X, in GoLib's space.
    harmonics = []
    for k, p in enumerate(found):
        rows = slice(starts[k], starts[k] + per)
        ways = positions[rows] - np.array(p)
        ways /= np.linalg.norm(ways, axis=1, keepdims=True)
        gx, gy, gz = ways[:, 0], ways[:, 2], -ways[:, 1]
        basis = np.stack([np.full_like(gx, 0.282095), 0.488603 * gy, 0.488603 * gz, 0.488603 * gx], axis=1)
        fit, *_ = np.linalg.lstsq(basis, light[rows], rcond=None)
        harmonics.append([round(float(v), 5) for v in fit.reshape(-1)])
    out = []
    it = iter(harmonics)
    for p in points:
        out.append(None if p is None else next(it))
    return origin, spacing, counts, out


def reflection_probes(scene):
    """The empties marked as reflection probes: custom property reflection."""
    return [o for o in scene.objects if o.type == 'EMPTY' and o.get("reflection")]


def render_reflections(scene, probes, folder, stem, width, samples):
    """Renders a panorama from the middle of each probe's box, of the level
    as it is lit, and returns them as GoLib reads them: where each is, its
    box, and its file."""
    found = []
    camera = bpy.data.objects.new("GoLib reflection", bpy.data.cameras.new("GoLib reflection"))
    scene.collection.objects.link(camera)
    camera.data.type = 'PANO'
    camera.data.panorama_type = 'EQUIRECTANGULAR'
    # Turned so the panorama's middle faces GoLib's +Z, Blender's -Y, with
    # the sky at its top, as GoLib reads an HDRI.
    camera.rotation_euler = (math.radians(90), 0, math.radians(180))
    keep = (scene.camera, scene.render.resolution_x, scene.render.resolution_y, scene.render.resolution_percentage,
            scene.render.filepath, scene.render.image_settings.file_format, scene.view_settings.view_transform,
            scene.view_settings.look, scene.cycles.samples, scene.cycles.use_denoising)
    moving = [o for o in scene.objects if o.type == 'MESH' and "static" in o.keys() and not o["static"] and not o.hide_render]
    try:
        for o in moving:
            o.hide_render = True
        scene.camera = camera
        scene.render.resolution_x, scene.render.resolution_y = width, width // 2
        scene.render.resolution_percentage = 100
        scene.render.image_settings.file_format = 'HDR'
        scene.view_settings.view_transform = 'Standard'
        scene.view_settings.look = 'None'
        scene.cycles.samples = samples
        scene.cycles.use_denoising = True
        for k, probe in enumerate(probes):
            center = probe.matrix_world.translation.copy()
            camera.location = center
            name = f"{stem}.reflection{k + 1}.hdr"
            scene.render.filepath = os.path.join(folder, name)
            bpy.ops.render.render(write_still=True)
            half = Vector([abs(s) * probe.empty_display_size for s in probe.matrix_world.to_scale()])
            g, h = to_golib(center), Vector((half.x, half.z, half.y))
            found.append({
                "file": name, "position": [round(v, 4) for v in g],
                "lo": [round(v, 4) for v in g - h], "hi": [round(v, 4) for v in g + h],
            })
    finally:
        (scene.camera, scene.render.resolution_x, scene.render.resolution_y, scene.render.resolution_percentage,
         scene.render.filepath, scene.render.image_settings.file_format, scene.view_settings.view_transform,
         scene.view_settings.look, scene.cycles.samples, scene.cycles.use_denoising) = keep
        for o in moving:
            o.hide_render = False
        bpy.data.objects.remove(camera)
    return found


def bake_into(objects, image, direct, indirect):
    scene = bpy.context.scene
    scene.render.bake.use_pass_direct = direct
    scene.render.bake.use_pass_indirect = indirect
    scene.render.bake.use_pass_color = False
    bpy.ops.object.select_all(action='DESELECT')
    for o in objects:
        o.select_set(True)
    bpy.context.view_layer.objects.active = objects[0]
    bpy.ops.object.bake(type='DIFFUSE')
    pixels = np.empty(len(image.pixels), dtype=np.float32)
    image.pixels.foreach_get(pixels)
    return pixels


# DENOISE runs in a Blender of its own, so the level's scene is left alone:
# the compositor's Denoise node, Intel's Open Image Denoise, smooths the
# grain of a bake as it does a render's, keeping how bright it is.
DENOISE = """
import bpy, sys
source, target = sys.argv[sys.argv.index("--") + 1:]
image = bpy.data.images.load(source)
scene = bpy.context.scene
scene.render.resolution_x, scene.render.resolution_y = image.size
scene.render.resolution_percentage = 100
scene.render.engine = 'BLENDER_WORKBENCH'
scene.render.use_compositing = True
scene.render.dither_intensity = 0
scene.view_settings.view_transform = 'Standard'
scene.render.image_settings.file_format = 'HDR'
scene.render.filepath = target
camera = bpy.data.objects.new("camera", bpy.data.cameras.new("camera"))
scene.collection.objects.link(camera)
scene.camera = camera
scene.use_nodes = True
tree = scene.node_tree
for node in list(tree.nodes):
    tree.nodes.remove(node)
picture = tree.nodes.new("CompositorNodeImage")
picture.image = image
denoise = tree.nodes.new("CompositorNodeDenoise")
denoise.use_hdr = True
out = tree.nodes.new("CompositorNodeComposite")
tree.links.new(picture.outputs["Image"], denoise.inputs["Image"])
tree.links.new(denoise.outputs["Image"], out.inputs["Image"])
bpy.ops.render.render(write_still=True)
"""


def denoise(source, target):
    done = subprocess.run(
        [bpy.app.binary_path, "--background", "--factory-startup", "--python-expr", DENOISE, "--", source, target],
        capture_output=True, text=True,
    )
    if done.returncode != 0 or not os.path.exists(target):
        fail("denoising the lightmap failed (run again with --no-denoise to keep the grain):\n" + done.stdout[-2000:] + done.stderr[-2000:])


def main():
    args = parse_arguments()
    began = time.time()
    scene = bpy.context.scene
    output = os.path.abspath(args.output)
    folder = os.path.dirname(output)
    lightmap_name = os.path.splitext(os.path.basename(output))[0] + ".lightmap.hdr"
    os.makedirs(folder, exist_ok=True)
    if bpy.context.object and bpy.context.object.mode != 'OBJECT':
        bpy.ops.object.mode_set(mode='OBJECT')

    objects = still_objects(scene)
    if not objects:
        fail("nothing to bake: the scene has no visible meshes, or every one has static = false")
    # Objects sharing a mesh would share its lightmap; each gets its own.
    for o in objects:
        if o.data.users > 1:
            o.data = o.data.copy()
    unwrap(objects, args.margin / args.size)

    scene.render.engine = 'CYCLES'
    scene.cycles.samples = args.samples
    scene.cycles.device = 'GPU' if args.gpu else 'CPU'
    scene.render.bake.margin = args.margin
    scene.render.bake.margin_type = 'EXTEND'
    image = bpy.data.images.new("GoLib lightmap", args.size, args.size, float_buffer=True)
    # Light as Cycles works it out, linear, saved as it is: an image in a
    # color space would be turned into the screen's colors on the way out.
    image.colorspace_settings.name = 'Non-Color'
    bake = Bake(objects, image)

    try:
        light = two_halves(scene, lambda direct, indirect: bake_into(objects, image, direct, indirect))
    finally:
        bake.remove()
    light[3::4] = 1.0
    image.pixels.foreach_set(light)
    image.file_format = 'HDR'
    target = os.path.join(folder, lightmap_name)
    if args.no_denoise:
        image.filepath_raw = target
        image.save()
    else:
        with tempfile.TemporaryDirectory() as scratch:
            image.filepath_raw = os.path.join(scratch, "baked.hdr")
            image.save()
            denoise(image.filepath_raw, target)
    bpy.data.images.remove(image)

    stem = os.path.splitext(os.path.basename(output))[0]
    probes_name = stem + ".probes.json"
    probes_file = {}
    probes_line = "no light probes"
    if not args.no_probes:
        origin, spacing, counts, harmonics = bake_probes(scene, objects, args.probe_spacing)
        probes_file.update({"origin": [round(v, 4) for v in origin], "spacing": round(spacing, 4), "size": counts, "probes": harmonics})
        inside = sum(h is None for h in harmonics)
        probes_line = f"{len(harmonics) - inside} light probes {spacing:g} m apart into {probes_name} ({inside} inside walls left out)"
    boxes = reflection_probes(scene)
    if boxes:
        probes_file["reflections"] = render_reflections(scene, boxes, folder, stem, args.reflection_size, args.samples)
    marker = None
    if probes_file:
        with open(os.path.join(folder, probes_name), "w") as f:
            json.dump(probes_file, f, separators=(",", ":"))
        marker = bpy.data.objects.new("GoLib light probes", None)
        marker["lightprobes"] = probes_name
        scene.collection.objects.link(marker)

    for o in objects:
        o["lightmap"] = lightmap_name
    bpy.ops.object.select_all(action='DESELECT')
    try:
        bpy.ops.export_scene.gltf(
            filepath=output, export_format='GLB', export_extras=True, export_yup=True,
            export_lights=True, export_import_convert_lighting_mode='RAW', use_active_scene=True,
        )
    finally:
        if marker is not None:
            bpy.data.objects.remove(marker)
    if args.save:
        bpy.context.preferences.filepaths.save_version = 0
        bpy.ops.wm.save_mainfile()
    smoothed = "" if args.no_denoise else ", denoised"
    print(f"[ok] baked {len(objects)} objects into {lightmap_name}, {args.size} by {args.size}, {args.samples} samples{smoothed}, in {time.time() - began:.0f} s")
    print(f"[ok] baked {probes_line}")
    print(f"[ok] rendered {len(boxes)} reflection probes" + (" (none: mark an empty over a room with the custom property reflection)" if not boxes else f", {args.reflection_size} by {args.reflection_size // 2}"))
    lamps = [o for o in scene.objects if o.type == 'LIGHT' and o.data.type in ('POINT', 'SPOT') and not o.hide_render]
    print(f"[ok] exported {output}, with its sun and {len(lamps)} lamps for Model.Sun and Model.Lights")


main()
