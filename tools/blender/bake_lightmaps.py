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
# Every still object gets a second UV map, Lightmap, unwrapped and packed for
# all of them together, as dense on a big wall as on a small crate, unless
# each has one already: then it is used as it is, so a hand-made one stays.
# Each object's custom property lightmap names the file, which GoLib reads.

import argparse
import math
import os
import subprocess
import sys
import tempfile
import time

import bpy
import numpy as np

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

    lamps = [o for o in scene.objects if o.type == 'LIGHT' and not o.hide_render]
    realtime = [o for o in lamps if o.data.type in REALTIME_LAMPS]
    baked_lamps = [o for o in lamps if o.data.type not in REALTIME_LAMPS]
    world = scene.world
    try:
        # The sky, glowing materials and area lamps: all their light.
        for o in realtime:
            o.hide_render = True
        around = bake_into(objects, image, direct=True, indirect=True)
        for o in realtime:
            o.hide_render = False
        # The sun and the lamps: only what they bounce, since GoLib lights
        # with them directly.
        bounced = np.zeros_like(around)
        if realtime:
            for o in baked_lamps:
                o.hide_render = True
            scene.world = None
            glow = [(s, s.default_value) for s in emissions()]
            for s, _ in glow:
                s.default_value = 0.0
            try:
                bounced = bake_into(objects, image, direct=False, indirect=True)
            finally:
                for s, value in glow:
                    s.default_value = value
                scene.world = world
                for o in baked_lamps:
                    o.hide_render = False
    finally:
        bake.remove()

    light = around + bounced
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

    for o in objects:
        o["lightmap"] = lightmap_name
    bpy.ops.object.select_all(action='DESELECT')
    bpy.ops.export_scene.gltf(
        filepath=output, export_format='GLB', export_extras=True, export_yup=True,
        export_lights=True, export_import_convert_lighting_mode='RAW', use_active_scene=True,
    )
    if args.save:
        bpy.context.preferences.filepaths.save_version = 0
        bpy.ops.wm.save_mainfile()
    smoothed = "" if args.no_denoise else ", denoised"
    print(f"[ok] baked {len(objects)} objects into {lightmap_name}, {args.size} by {args.size}, {args.samples} samples{smoothed}, in {time.time() - began:.0f} s")
    print(f"[ok] exported {output}, with its sun and {len(realtime) - sum(o.data.type == 'SUN' for o in realtime)} lamps for Model.Sun and Model.Lights")


main()
