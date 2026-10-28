# Builds Lantern's level, a flooded temple of three stone halls, in Blender
# and saves it as level.blend next to this file. It doesn't export: the
# level's light is baked, and tools/blender/bake_lightmaps.py exports it with
# its lightmap. Run both from the GoLib folder, with Blender in the
# background (/Applications/Blender.app/Contents/MacOS/Blender on macOS):
#
#   blender --background --factory-startup --python examples/lantern/sources/models/level.py
#   blender --background examples/lantern/sources/models/level.blend --python tools/blender/bake_lightmaps.py -- \
#       --output examples/lantern/assets/models/level.glb
#
# It also paints the level's textures, sources/models/textures/stone.png and
# marble.png, which the .blend and the .glb carry.
#
# The level, from the entrance along the game's +Z:
#   - The hall of pillars: polished marble floor, four pillars, two torches.
#   - The flooded hall: a sunken pool in the middle, a ledge around it and
#     a deeper middle; the roof over it is broken, so moonlight falls through
#     the gaps; four torches and four pillars.
#   - The hall of the gate: polished floor, four pillars, two torches, and
#     the gate in its far wall, which opens onto the way out.
#
# Empties tell the game where things go, with the custom property "kind":
#   start   where the explorer starts, facing the way it points
#   gem     a gem to collect
#   torch   a flame on a torch: the game draws its fire and smoke
#   gate    the gate, a cube scaled to its size: the game draws it and
#           opens it
#   exit    the way out, behind the gate
# and three more, kind "room", with the custom property reflection = true,
# are the reflection probes the bake renders, one over each hall.
#
# Positions below are written as the game sees them, (x, up, z) in meters,
# with z the game's +Z, Blender's -Y.

import math
import os
import sys

import bmesh
import bpy
import numpy as np
from mathutils import Matrix, Vector

sys.dont_write_bytecode = True

HERE = os.path.dirname(os.path.abspath(__file__))
BLEND = os.path.join(HERE, "level.blend")
TEXTURES = os.path.join(HERE, "textures")

TILE = 2.0          # meters a texture covers before it repeats
HEIGHT = 5.0        # from the floor to the roof
WALL = 0.6          # walls' thickness
DOOR_W = 2.4        # doorways' width
DOOR_H = 3.2        # doorways' height
FLOOR_DEPTH = 0.4   # floors' thickness, below 0

# Halls: (x0, x1, z0, z1), the inside of each.
HALL_A = (-5.0, 5.0, 0.0, 11.7)
HALL_B = (-7.0, 7.0, 12.3, 27.7)
HALL_C = (-5.0, 5.0, 28.3, 40.0)
ALCOVE = (-1.8, 1.8, 40.6, 43.6)  # the way out, behind the gate

# The pool in the flooded hall: the hole in its floor, the ledge around its
# middle and the middle, with their tops.
POOL = (-4.0, 4.0, 15.0, 25.0)
LEDGE_TOP = -0.25
POOL_MIDDLE = (-3.2, 3.2, 15.8, 24.2)
POOL_BOTTOM = -0.5

# The gaps in the flooded hall's roof, across x from GAP_X0 to GAP_X1.
GAP_X0, GAP_X1 = -5.0, 3.5
GAPS = [(14.8, 16.4), (17.8, 19.4), (20.8, 22.4), (23.8, 25.4)]

PILLARS = [(-2.8, 4.0), (2.8, 4.0), (-2.8, 8.5), (2.8, 8.5),
           (-5.6, 14.6), (5.6, 14.6), (-5.6, 25.4), (5.6, 25.4),
           (-2.8, 31.5), (2.8, 31.5), (-2.8, 36.5), (2.8, 36.5)]
PILLAR = 0.8

# Torches: where the wall is (x of its face, z) and which way it faces, +1
# for +X, -1 for -X.
TORCHES = [(-5.0, 6.25, 1), (5.0, 6.25, -1),
           (-7.0, 17.5, 1), (7.0, 17.5, -1), (-7.0, 22.5, 1), (7.0, 22.5, -1),
           (-5.0, 34.0, 1), (5.0, 34.0, -1)]
TORCH_UP = 2.2      # the bowl's top
TORCH_OUT = 0.35    # how far the bowl stands from the wall
TORCH_WATTS = 50
TORCH_COLOR = (1.0, 0.5, 0.18)

GEMS = [(0.0, 0.9, 7.4), (-4.0, 0.9, 10.6),
        (0.0, 0.7, 20.0), (6.0, 0.9, 26.6),
        (-4.0, 0.9, 30.0), (3.8, 0.9, 38.6)]

START = ((0.0, 0.0, 4.4), 0)
GATE = ((0.0, DOOR_H / 2, 40.3), (DOOR_W, DOOR_H, WALL))
EXIT = (0.0, 0.0, 42.6)

# The moon: the way its light travels, in the game's axes.
MOON_DIRECTION = (0.3, -1.0, 0.45)
MOON_STRENGTH = 7.5
MOON_COLOR = (0.62, 0.72, 1.0)
NIGHT_SKY = (0.05, 0.07, 0.16)


# ---------------------------------------------------------------------------
# Textures, painted with numpy.


def save_image(name, rgb):
    """Saves an (n, n, 3) array, row 0 at the bottom, as textures/<name>.png
    and returns the image, read from there: saving the .blend makes the path
    relative to it, so the .blend holds no folder of this machine's."""
    size = rgb.shape[0]
    image = bpy.data.images.new(name, size, size, alpha=False)
    rgba = np.ones((size, size, 4), dtype=np.float32)
    rgba[:, :, :3] = np.clip(rgb, 0, 1)
    image.pixels.foreach_set(rgba.ravel())
    os.makedirs(TEXTURES, exist_ok=True)
    image.filepath_raw = os.path.join(TEXTURES, name + ".png")
    image.file_format = 'PNG'
    image.save()
    return image


def stone_texture(size=256):
    """Rough sandstone blocks, four across and eight up a tile, every other
    row shifted half a block, in dark joints."""
    rng = np.random.default_rng(11)
    y, x = np.mgrid[0:size, 0:size]
    bw, bh = size // 4, size // 8
    row = y // bh
    shifted = x + np.where(row % 2 == 1, bw // 2, 0)
    col = (shifted // bw) % 4
    bx, by = shifted % bw, y % bh
    shade = rng.uniform(0.78, 1.15, size=(8, 4))[row, col]
    grain = rng.uniform(-1, 1, size=(size, size)) * 0.04
    base = 0.46 * shade + grain
    joint = (bx < 3) | (by < 3)
    rgb = np.stack([base, base * 0.92, base * 0.80], axis=2)
    rgb[joint] = [0.12, 0.11, 0.10]
    return rgb


def marble_texture(size=256):
    """Polished marble tiles, two by two a tile, alternately dark and pale,
    with a soft vein, in thin dark joints."""
    rng = np.random.default_rng(21)
    y, x = np.mgrid[0:size, 0:size].astype(np.float32)
    cell = size // 2
    checker = ((x // cell + y // cell) % 2).astype(np.float32)
    vein = 0.5 + 0.5 * np.sin((x * 0.9 + y * 0.6) / 9 + 3 * np.sin(y / 23 + x / 41))
    vein = vein ** 6 * 0.12
    grain = rng.uniform(-1, 1, size=(size, size)) * 0.015
    dark = np.array([0.10, 0.11, 0.13])
    pale = np.array([0.42, 0.40, 0.37])
    rgb = dark + (pale - dark) * checker[..., None]
    rgb = rgb + (vein + grain)[..., None]
    joint = ((x % cell) < 2) | ((y % cell) < 2)
    rgb[joint] = [0.05, 0.05, 0.05]
    return rgb


def pool_texture(size=128):
    """The pool's mossy stone, flagstones a tile apart, greener in the
    cracks."""
    rng = np.random.default_rng(31)
    y, x = np.mgrid[0:size, 0:size]
    cell = size // 2
    shade = rng.uniform(0.8, 1.1, size=(2, 2))[y // cell, x // cell]
    grain = rng.uniform(-1, 1, size=(size, size)) * 0.05
    base = 0.22 * shade + grain
    rgb = np.stack([base * 0.8, base, base * 0.75], axis=2)
    joint = ((x % cell) < 3) | ((y % cell) < 3)
    rgb[joint] = [0.05, 0.12, 0.06]
    return rgb


def material(name, image=None, color=(1, 1, 1, 1), roughness=0.9, metallic=0.0, emission=None, emission_strength=0.0):
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs["Base Color"].default_value = color
    bsdf.inputs["Roughness"].default_value = roughness
    bsdf.inputs["Metallic"].default_value = metallic
    if emission is not None:
        bsdf.inputs["Emission Color"].default_value = emission
        bsdf.inputs["Emission Strength"].default_value = emission_strength
    if image is not None:
        tex = mat.node_tree.nodes.new("ShaderNodeTexImage")
        tex.image = image
        mat.node_tree.links.new(tex.outputs["Color"], bsdf.inputs["Base Color"])
    return mat


# ---------------------------------------------------------------------------
# Geometry: boxes gathered into one mesh per material, with texture
# coordinates from where each face is in the world, so the stone runs on
# across boxes without seams.


def blender(x, up, z):
    """The game's (x, up, z) in Blender's axes: the game's z is Blender's -Y."""
    return Vector((x, -z, up))


class Builder:
    def __init__(self, name):
        self.name = name
        self.bm = bmesh.new()

    def box(self, x0, x1, y0, y1, z0, z1):
        """A box from the game's corners (x0, y0, z0) to (x1, y1, z1)."""
        if x1 - x0 <= 0.001 or y1 - y0 <= 0.001 or z1 - z0 <= 0.001:
            return
        center = blender((x0 + x1) / 2, (y0 + y1) / 2, (z0 + z1) / 2)
        size = Vector((x1 - x0, z1 - z0, y1 - y0))
        bmesh.ops.create_cube(self.bm, size=1.0, matrix=Matrix.LocRotScale(center, None, size))

    def cylinder(self, x, y0, y1, z, radius, top_radius=None, segments=12):
        top_radius = radius if top_radius is None else top_radius
        center = blender(x, (y0 + y1) / 2, z)
        bmesh.ops.create_cone(self.bm, cap_ends=True, segments=segments, radius1=radius, radius2=top_radius,
                              depth=y1 - y0, matrix=Matrix.Translation(center))

    def finish(self, mat, collide=True, tile=TILE):
        bm = self.bm
        bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
        uv = bm.loops.layers.uv.new("UVMap")
        for face in bm.faces:
            n = face.normal
            for loop in face.loops:
                co = loop.vert.co
                if abs(n.z) >= abs(n.x) and abs(n.z) >= abs(n.y):
                    loop[uv].uv = (co.x / tile, co.y / tile)
                elif abs(n.x) >= abs(n.y):
                    loop[uv].uv = (co.y / tile, co.z / tile)
                else:
                    loop[uv].uv = (co.x / tile, co.z / tile)
        mesh = bpy.data.meshes.new(self.name)
        bm.to_mesh(mesh)
        bm.free()
        obj = bpy.data.objects.new(self.name, mesh)
        obj.data.materials.append(mat)
        if not collide:
            obj["collide"] = False
        bpy.context.scene.collection.objects.link(obj)
        return obj


def walls_around(walls, hall, openings):
    """The walls of a hall, outside its inside, with doorways: openings maps
    a side, "near" (low z) or "far" (high z), to the doorway's x range, or
    to None for no wall there, where the next hall's wall stands."""
    x0, x1, z0, z1 = hall
    walls.box(x0 - WALL, x0, 0, HEIGHT, z0, z1)
    walls.box(x1, x1 + WALL, 0, HEIGHT, z0, z1)
    for side, z in (("near", (z0 - WALL, z0)), ("far", (z1, z1 + WALL))):
        if side in openings and openings[side] is None:
            continue
        if side in openings:
            a, b = openings[side]
            walls.box(x0 - WALL, a, 0, HEIGHT, *z)
            walls.box(b, x1 + WALL, 0, HEIGHT, *z)
            walls.box(a, b, DOOR_H, HEIGHT, *z)
        else:
            walls.box(x0 - WALL, x1 + WALL, 0, HEIGHT, *z)


def build_level(images):
    stone = material("Stone", images["stone"], roughness=0.9)
    marble = material("Marble", images["marble"], roughness=0.12)
    mossy = material("Pool stone", images["pool"], roughness=0.7)
    iron = material("Iron", color=(0.06, 0.05, 0.045, 1), roughness=0.45, metallic=0.9)
    coals = material("Coals", color=(0.05, 0.02, 0.01, 1), roughness=0.9,
                     emission=(1.0, 0.35, 0.08, 1), emission_strength=6.0)

    door = (-DOOR_W / 2, DOOR_W / 2)
    walls = Builder("Walls")
    # The separating walls are shared: the hall of pillars has its far wall
    # wide enough for the flooded hall's near side, and so on.
    walls_around(walls, HALL_A, {"far": None})
    walls_around(walls, HALL_B, {"near": door, "far": door})
    walls_around(walls, HALL_C, {"near": None, "far": door})
    ax0, ax1, az0, az1 = ALCOVE
    walls.box(ax0 - WALL, ax0, 0, HEIGHT, az0, az1)
    walls.box(ax1, ax1 + WALL, 0, HEIGHT, az0, az1)
    walls.box(ax0 - WALL, ax1 + WALL, 0, HEIGHT, az1, az1 + WALL)
    for x, z in PILLARS:
        h = PILLAR / 2
        walls.box(x - h, x + h, 0, HEIGHT, z - h, z + h)
        walls.box(x - h - 0.12, x + h + 0.12, 0, 0.35, z - h - 0.12, z + h + 0.12)  # plinth
    walls.finish(stone)

    roof = Builder("Roof")
    for x0, x1, z0, z1 in (HALL_A, HALL_C, ALCOVE):
        roof.box(x0 - WALL, x1 + WALL, HEIGHT, HEIGHT + 0.5, z0 - WALL, z1 + WALL)
    bx0, bx1, bz0, bz1 = HALL_B
    roof.box(bx0 - WALL, GAP_X0, HEIGHT, HEIGHT + 0.5, bz0, bz1)
    roof.box(GAP_X1, bx1 + WALL, HEIGHT, HEIGHT + 0.5, bz0, bz1)
    z = bz0
    for g0, g1 in GAPS:
        roof.box(GAP_X0, GAP_X1, HEIGHT, HEIGHT + 0.5, z, g0)
        z = g1
    roof.box(GAP_X0, GAP_X1, HEIGHT, HEIGHT + 0.5, z, bz1)
    roof.finish(stone)

    floor = Builder("Floor")
    for x0, x1, z0, z1 in (HALL_A, HALL_C, ALCOVE):
        floor.box(x0, x1, -FLOOR_DEPTH, 0, z0, z1)
    # The doorways' thresholds.
    for z in (HALL_A[3], HALL_B[3], HALL_C[3]):
        floor.box(-DOOR_W / 2, DOOR_W / 2, -FLOOR_DEPTH, 0, z, z + WALL)
    # The flooded hall, around the pool's hole.
    px0, px1, pz0, pz1 = POOL
    floor.box(bx0, bx1, -FLOOR_DEPTH, 0, bz0, pz0)
    floor.box(bx0, bx1, -FLOOR_DEPTH, 0, pz1, bz1)
    floor.box(bx0, px0, -FLOOR_DEPTH, 0, pz0, pz1)
    floor.box(px1, bx1, -FLOOR_DEPTH, 0, pz0, pz1)
    floor.finish(marble)

    pool = Builder("Pool")
    mx0, mx1, mz0, mz1 = POOL_MIDDLE
    deep = -FLOOR_DEPTH - 0.5
    pool.box(px0, px1, deep, LEDGE_TOP, pz0, mz0)
    pool.box(px0, px1, deep, LEDGE_TOP, mz1, pz1)
    pool.box(px0, mx0, deep, LEDGE_TOP, mz0, mz1)
    pool.box(mx1, px1, deep, LEDGE_TOP, mz0, mz1)
    pool.box(mx0, mx1, deep, POOL_BOTTOM, mz0, mz1)
    pool.finish(mossy)

    torches = Builder("Torch brackets")
    bowls = Builder("Torch coals")
    for wx, z, facing in TORCHES:
        out = wx + facing * TORCH_OUT
        a, b = sorted((wx, out))
        torches.box(a, b, TORCH_UP - 0.45, TORCH_UP - 0.37, z - 0.04, z + 0.04)   # arm
        torches.box(wx, wx + facing * 0.05, TORCH_UP - 0.6, TORCH_UP - 0.2, z - 0.1, z + 0.1)  # plate
        torches.cylinder(out, TORCH_UP - 0.5, TORCH_UP - 0.15, z, 0.035)       # handle
        torches.cylinder(out, TORCH_UP - 0.18, TORCH_UP, z, 0.06, 0.13)         # bowl
        bowls.cylinder(out, TORCH_UP - 0.02, TORCH_UP + 0.02, z, 0.11)          # glowing coals
    torches.finish(iron, collide=False)
    bowls.finish(coals, collide=False)


# ---------------------------------------------------------------------------
# Lamps and empties.


def lamps():
    world = bpy.data.worlds.new("Night")
    world.use_nodes = True
    background = world.node_tree.nodes["Background"]
    background.inputs["Color"].default_value = (*NIGHT_SKY, 1)
    background.inputs["Strength"].default_value = 1.0
    bpy.context.scene.world = world

    moon = bpy.data.lights.new("Moon", 'SUN')
    moon.energy = MOON_STRENGTH
    moon.color = MOON_COLOR
    moon.angle = math.radians(1.0)
    obj = bpy.data.objects.new("Moon", moon)
    way = blender(*MOON_DIRECTION).normalized()
    obj.rotation_euler = way.to_track_quat('-Z', 'Y').to_euler()
    obj.location = blender(0, 12, 20)
    bpy.context.scene.collection.objects.link(obj)

    for i, (wx, z, facing) in enumerate(TORCHES):
        light = bpy.data.lights.new("Torch.%02d" % (i + 1), 'POINT')
        light.energy = TORCH_WATTS
        light.color = TORCH_COLOR
        light.shadow_soft_size = 0.08
        obj = bpy.data.objects.new("Torch light.%02d" % (i + 1), light)
        # A little above the flame and further from the wall than the bowl,
        # so the bowl doesn't shadow the whole floor.
        obj.location = blender(wx + facing * (TORCH_OUT + 0.15), TORCH_UP + 0.35, z)
        bpy.context.scene.collection.objects.link(obj)


def empty(name, kind, position, yaw=0, display='PLAIN_AXES', size=0.3, scale=None, **props):
    obj = bpy.data.objects.new(name, None)
    obj.empty_display_type = display
    obj.empty_display_size = size
    obj.location = blender(*position)
    obj.rotation_euler = (0, 0, math.radians(yaw))
    if scale is not None:
        w, h, d = scale
        obj.scale = (w, d, h)
    obj["kind"] = kind
    for key, value in props.items():
        obj[key] = value
    bpy.context.scene.collection.objects.link(obj)
    return obj


def place_markers():
    at, yaw = START
    empty("Start", "start", at, yaw, 'SINGLE_ARROW', 1.0)
    empty("Exit", "exit", EXIT, 0, 'SPHERE', 0.6)
    gate_at, gate_size = GATE
    empty("Gate", "gate", gate_at, 0, 'CUBE', 0.5, scale=gate_size)
    for i, p in enumerate(GEMS):
        empty("Gem.%d" % (i + 1), "gem", p, 0, 'SPHERE', 0.25)
    for i, (wx, z, facing) in enumerate(TORCHES):
        empty("Flame.%02d" % (i + 1), "torch", (wx + facing * TORCH_OUT, TORCH_UP, z), 0, 'CONE', 0.15)
    # Reflection probes: cubes of size 1 scaled to each hall's half size.
    for name, (x0, x1, z0, z1) in (("Hall of pillars", HALL_A), ("Flooded hall", HALL_B), ("Hall of the gate", HALL_C)):
        center = ((x0 + x1) / 2, HEIGHT / 2, (z0 + z1) / 2)
        empty("Room." + name, "room", center, 0, 'CUBE', 1.0,
              scale=((x1 - x0) / 2, HEIGHT / 2, (z1 - z0) / 2), reflection=True)


def main():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.context.preferences.filepaths.save_version = 0  # no level.blend1 backups
    bpy.context.scene.name = "Lantern"
    images = {
        "stone": save_image("stone", stone_texture()),
        "marble": save_image("marble", marble_texture()),
        "pool": save_image("pool", pool_texture()),
    }
    build_level(images)
    lamps()
    place_markers()
    bpy.ops.wm.save_as_mainfile(filepath=BLEND, relative_remap=True)
    print("saved", BLEND)


main()
