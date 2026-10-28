# Builds Crypt's level in Blender, saves it as level.blend next to this file
# and exports it to the game's assets/models/level.glb. It also paints the
# textures the level and the game use, so everything the crypt looks like is
# made here: sources/models/textures/wall.png and floor.png, which the level
# carries inside the .glb, and assets/textures/crate.png and door.png, which
# the game draws on its crates and doors.
#
# Run it from the GoLib folder, with Blender in the background:
#
#   /Applications/Blender.app/Contents/MacOS/Blender --background --factory-startup --python examples/crypt/sources/models/level.py
#
# (Blender's own folder under Program Files on Windows). It overwrites
# level.blend and level.glb: change the numbers below and run it again, or
# open level.blend, move things, and export it with File > Export > glTF 2.0
# (glTF Binary, +Y Up, Include > Custom Properties, Active Scene).
#
# The level is made of:
#   - Solid stone, as meshes: the floors, the walls with their doorways, the
#     pillars, the stairs up to the great hall's gallery and the ramp down
#     from it, the gallery itself, the raised floor of the lift hall and of
#     the exit chamber, and two stone coffins.
#   - Torch brackets, a mesh with the custom property collide = false, so
#     nothing bumps into them.
#   - Empties that tell the game where things go, with the custom property
#     "kind" (read with golib's Model.Markers):
#       start       where the player starts, facing the way it points
#       checkpoint  where the player comes back after a guard grabs them;
#                   "room" names it
#       exit        the way out, at the far end
#       torch       a flame on a wall; the game draws it, lights it and
#                   makes it crackle
#       door        a locked door; the empty is a cube whose scale is the
#                   door's size, and the game draws it and makes it solid
#       crate       a crate, as a cube scaled to its size; "key" = 1 for the
#                   crate a key is hidden in
#       key         a key lying in the open
#       lift        the stone lift: a cube scaled to its size, at the bottom
#                   of its shaft; "rise" is how far up it goes, in meters
#       guard       where a skeleton guard starts; "route" names its patrol
#       patrol      a point of a patrol: "route" and "order", from 1
#
# Positions below are written as the game sees them: (x, up, z), in meters,
# with z the game's +Z (Blender's -Y) and up Blender's +Z. The crypt runs
# north along +Z: the entry hall, the great hall with its gallery, the lift
# hall to the east, and the exit chamber on the lift hall's upper floor.

import math
import os
import random
import sys

import bmesh
import bpy
from mathutils import Matrix, Vector

sys.dont_write_bytecode = True

HERE = os.path.dirname(os.path.abspath(__file__))
GAME = os.path.dirname(os.path.dirname(HERE))
BLEND = os.path.join(HERE, "level.blend")
GLB = os.path.join(GAME, "assets", "models", "level.glb")
SOURCE_TEXTURES = os.path.join(HERE, "textures")
GAME_TEXTURES = os.path.join(GAME, "assets", "textures")

T = 0.5         # meters: the walls' thickness
TILE = 2.0      # meters a texture covers before it repeats

# ---------------------------------------------------------------------------
# The plan. Rooms are (x0, x1, z0, z1, floor, wall top): the inside of each,
# walls stand outside it.

ROOM_A = (-5, 5, 0, 9, 0, 3.2)            # the entry hall
CORRIDOR_1 = (-1.2, 1.2, 9.5, 14, 0, 3.2)   # entry hall to great hall
ROOM_B = (-8, 8, 14.5, 30, 0, 6.5)          # the great hall
CORRIDOR_2 = (8.5, 13, 14.7, 16.7, 0, 3.2)  # great hall to lift hall
ROOM_C = (13.5, 23.5, 10, 26, 0, 8.5)       # the lift hall
UPPER = 4.0                                 # meters: the lift hall's upper floor and the exit chamber's
CORRIDOR_3 = (17.0, 19.4, 26.5, 30, UPPER, UPPER + 3.2)
ROOM_D = (13.5, 23.5, 30.5, 40, UPPER, UPPER + 3.8)  # the exit chamber

DOOR_HEIGHT = 2.3

# Heights the characters can't climb: GoLib's CharacterController steps up
# ledges as high as its StepHeight (0.27 m here) plus about its radius, so
# anything meant to stop it stands taller than 0.6 m.
PLINTH = 0.65   # a pillar's foot
PARAPET = 0.8   # the low walls along the gallery and the upper floor

# The great hall's gallery, up the stairs on the west and down the ramp on
# the east.
GALLERY_Z = 26.0       # where the gallery's floor begins, to the north wall
GALLERY_TOP = 2.5      # meters
GALLERY_SLAB = 0.3
STAIRS_X = (-8, -6.4)
STAIRS_Z0 = 17.0       # the first step
STEPS = 10             # each GALLERY_TOP / STEPS high
RAMP_X = (6.4, 8)
RAMP_Z0 = 17.6         # the foot of the ramp

# The lift hall's upper floor, and the lift beside it.
UPPER_Z = 20.0         # where the upper floor begins, to the north wall
LIFT = ((18.2, 0.12, 18.8), (2.4, 0.24, 2.4))  # (x, up, z) of its middle at the bottom, and its size: a step up
LIFT_RISE = UPPER - 0.24

PILLARS = [
    # (x, z, size, height)
    (-2.5, 4.5, 0.8, ROOM_A[5]), (2.5, 4.5, 0.8, ROOM_A[5]),
    (-2.6, 19.0, 0.9, ROOM_B[5]), (2.6, 19.0, 0.9, ROOM_B[5]),
    (-2.6, 23.0, 0.9, ROOM_B[5]), (2.6, 23.0, 0.9, ROOM_B[5]),
    (-3.5, GALLERY_Z + 0.3, 0.5, GALLERY_TOP - GALLERY_SLAB),  # under the gallery's edge
    (3.5, GALLERY_Z + 0.3, 0.5, GALLERY_TOP - GALLERY_SLAB),
    (16.2, 14.0, 0.8, ROOM_C[5]), (20.6, 14.0, 0.8, ROOM_C[5]),
]

# Stone coffins in the exit chamber: (x, z, width, length), on its floor.
COFFINS = [(15.6, 35.0, 1.1, 2.3), (21.4, 35.0, 1.1, 2.3)]

START = ((0, 0, 3.0), 0)      # (x, up, z) and yaw: facing north
EXIT = (18.4, UPPER, 39.2)

CHECKPOINTS = [
    ("entry hall", (0, 0, 3.0), 0),
    ("great hall", (0, 0, 12.4), 0),
    ("lift hall", (11.0, 0, 15.7), 90),
    ("exit chamber", (18.2, UPPER, 27.6), 0),
]

DOORS = [
    # name, (x, up, z) of the middle, (width x, height, depth z)
    ("Door1", (0, DOOR_HEIGHT / 2, 9.25), (2.4, DOOR_HEIGHT, 0.3)),
    ("Door2", (8.25, DOOR_HEIGHT / 2, 15.7), (0.3, DOOR_HEIGHT, 2.0)),
    ("Door3", (18.2, UPPER + DOOR_HEIGHT / 2, 26.25), (2.4, DOOR_HEIGHT, 0.3)),
]

CRATE = 0.8  # meters a side
CRATES = [
    # name, (x, z), floor, hides a key
    ("Crate.A1", (-3.9, 7.8), 0, False),
    ("Crate.A2", (3.9, 7.9), 0, True),
    ("Crate.A3", (3.7, 1.0), 0, False),
    ("Crate.B1", (-1.0, 28.6), 0, False),  # under the gallery
    ("Crate.B2", (5.0, 15.4), 0, False),
    ("Crate.C1", (14.5, 11.0), 0, False),
    ("Crate.C2", (22.4, 11.0), 0, False),
    ("Crate.C3", (22.4, 12.0), 0, False),
    ("Crate.C4", (14.6, 19.0), 0, True),
    ("Crate.D1", (22.5, 31.4), UPPER, False),
]

KEYS = [("Key.Gallery", (0, GALLERY_TOP, 28.8))]

GUARDS = [
    # name, route, start (x, up, z), and the route's points
    ("Guard.Hall", "hall", [(-4.6, 0, 17.0), (4.6, 0, 17.0), (4.6, 0, 25.0), (-4.6, 0, 25.0)]),
    ("Guard.Lift", "lift", [(15.4, 0, 11.6), (21.6, 0, 11.6)]),  # back and forth along the south wall
    ("Guard.Exit", "exit", [(14.6, UPPER, 38.4), (22.4, UPPER, 38.4), (22.4, UPPER, 32.8), (14.6, UPPER, 32.8)]),
]

TORCH_HEIGHT = 1.9  # meters above the floor
TORCHES = [
    # (x, up, z) of the flame, and the way it faces out of the wall: n, s, e, w
    ((-5, TORCH_HEIGHT, 2.6), "e"), ((-5, TORCH_HEIGHT, 6.6), "e"),
    ((5, TORCH_HEIGHT, 2.6), "w"), ((5, TORCH_HEIGHT, 6.6), "w"),
    ((1.2, TORCH_HEIGHT, 11.8), "w"),
    ((-4.8, 2.2, 14.5), "n"), ((4.8, 2.2, 14.5), "n"),
    ((-4.0, GALLERY_TOP + TORCH_HEIGHT, 30), "s"), ((4.0, GALLERY_TOP + TORCH_HEIGHT, 30), "s"),
    ((-8, 4.2, 21.0), "e"), ((8, 4.2, 21.0), "w"),
    ((13.5, TORCH_HEIGHT, 12.4), "e"), ((23.5, TORCH_HEIGHT, 14.0), "w"), ((23.5, TORCH_HEIGHT, 18.4), "w"),
    ((13.5, UPPER + TORCH_HEIGHT, 23.0), "e"), ((23.5, UPPER + TORCH_HEIGHT, 23.0), "w"),
    ((13.5, UPPER + TORCH_HEIGHT, 33.0), "e"), ((13.5, UPPER + TORCH_HEIGHT, 37.5), "e"),
    ((23.5, UPPER + TORCH_HEIGHT, 33.0), "w"), ((23.5, UPPER + TORCH_HEIGHT, 37.5), "w"),
    ((16.6, UPPER + TORCH_HEIGHT, 40), "s"), ((20.2, UPPER + TORCH_HEIGHT, 40), "s"),
]

FACING = {"n": (0, 1), "s": (0, -1), "e": (1, 0), "w": (-1, 0)}  # (x, z) out of the wall

# ---------------------------------------------------------------------------
# Textures, painted pixel by pixel, the same every time.


def new_image(name, size):
    image = bpy.data.images.new(name, size, size, alpha=False)
    return image


def save_image(image, folder, name):
    os.makedirs(folder, exist_ok=True)
    path = os.path.join(folder, name)
    image.filepath_raw = path
    image.file_format = 'PNG'
    image.save()
    return path


def paint(size, color_at):
    """Returns Blender's flat RGBA list for a size by size picture, with
    color_at(x, y) giving (r, g, b) from 0 to 1, y up from the bottom."""
    pixels = [0.0] * (size * size * 4)
    for y in range(size):
        for x in range(size):
            r, g, b = color_at(x, y)
            i = (y * size + x) * 4
            pixels[i] = min(max(r, 0), 1)
            pixels[i + 1] = min(max(g, 0), 1)
            pixels[i + 2] = min(max(b, 0), 1)
            pixels[i + 3] = 1.0
    return pixels


def noise_table(size, seed):
    rng = random.Random(seed)
    return [rng.uniform(-1, 1) for _ in range(size * size)]


def wall_texture(size=256):
    """Rough stone blocks, 0.5 by 0.25 meters at TILE meters a picture, each
    row half a block along from the one under it, in dark mortar."""
    rng = random.Random(11)
    across, rows = 4, 8
    bw, bh = size // across, size // rows
    shade = {(c, r): rng.uniform(0.78, 1.12) for c in range(across + 1) for r in range(rows)}
    grain = noise_table(size, 12)

    def color_at(x, y):
        row = y // bh
        shifted = x + (bw // 2 if row % 2 else 0)
        col = (shifted // bw) % (across + 1)
        bx, by = shifted % bw, y % bh
        n = grain[y * size + x] * 0.045
        if bx < 3 or by < 3:
            return (0.13 + n, 0.12 + n, 0.11 + n)
        s = shade[(col if row % 2 == 0 else col, row)]
        edge = 0.9 if bx < 6 or by < 6 or bx > bw - 4 or by > bh - 4 else 1.0
        base = 0.40 * s * edge
        return (base + n, base * 0.96 + n, base * 0.9 + n)

    return paint(size, color_at)


def floor_texture(size=256):
    """Flagstones, 0.5 meters a side at TILE meters a picture, some worn
    darker, with dark joints."""
    rng = random.Random(21)
    across = 4
    cell = size // across
    shade = {(c, r): rng.uniform(0.75, 1.1) for c in range(across) for r in range(across)}
    grain = noise_table(size, 22)

    def color_at(x, y):
        c, r = x // cell, y // cell
        bx, by = x % cell, y % cell
        n = grain[y * size + x] * 0.05
        if bx < 3 or by < 3:
            return (0.10 + n, 0.09 + n, 0.085 + n)
        s = shade[(c, r)]
        base = 0.34 * s
        return (base + n, base * 0.97 + n, base * 0.93 + n)

    return paint(size, color_at)


def crate_texture(size=128):
    """A crate's side: planks inside a darker frame, with a brace across."""
    grain = noise_table(size, 31)
    frame = size // 8

    def color_at(x, y):
        n = grain[y * size + x] * 0.035
        wood = 0.42 + 0.05 * math.sin(y * 0.9 + math.sin(x * 0.21) * 3)
        plank = x // (size // 4)
        if x % (size // 4) < 2:
            wood *= 0.55
        wood *= (0.92, 1.0, 0.95, 1.05)[plank % 4]
        if x < frame or y < frame or x >= size - frame or y >= size - frame or abs(x - y) < frame // 2 + 1:
            wood = 0.30 + 0.03 * math.sin(x * 0.5 + y * 0.5)
            if x < 2 or y < 2 or x >= size - 2 or y >= size - 2:
                wood *= 0.5
        return (wood * 1.0 + n, wood * 0.68 + n, wood * 0.38 + n)

    return paint(size, color_at)


def door_texture(size=128):
    """A door: dark oak planks held by two iron bands with rivets."""
    grain = noise_table(size, 41)

    def color_at(x, y):
        n = grain[y * size + x] * 0.03
        wood = 0.30 + 0.04 * math.sin(x * 0.25 + math.sin(y * 0.15) * 2)
        if x % (size // 5) < 2:
            wood *= 0.5
        band = (size * 0.18 < y < size * 0.28) or (size * 0.72 < y < size * 0.82)
        if band:
            rivet = (x % (size // 5)) in (size // 10 - 1, size // 10, size // 10 + 1) and abs(y - (size * 0.23 if y < size / 2 else size * 0.77)) < 3
            iron = 0.30 if rivet else 0.16
            return (iron + n, iron + n, iron * 1.08 + n)
        return (wood * 1.0 + n, wood * 0.62 + n, wood * 0.36 + n)

    return paint(size, color_at)


def textures():
    """Paints every texture: the level's become images in the .blend, the
    game's are saved in its assets."""
    made = {}
    for name, painter, size, folder in (
        ("wall", wall_texture, 256, SOURCE_TEXTURES),
        ("floor", floor_texture, 256, SOURCE_TEXTURES),
        ("crate", crate_texture, 128, GAME_TEXTURES),
        ("door", door_texture, 128, GAME_TEXTURES),
    ):
        image = new_image(name, size)
        image.pixels[:] = painter(size)
        save_image(image, folder, name + ".png")
        made[name] = image
    return made


def material(name, image=None, color=(1, 1, 1, 1), roughness=0.9, metallic=0.0):
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs["Base Color"].default_value = color
    bsdf.inputs["Roughness"].default_value = roughness
    bsdf.inputs["Metallic"].default_value = metallic
    if image is not None:
        tex = mat.node_tree.nodes.new("ShaderNodeTexImage")
        tex.image = image
        mat.node_tree.links.new(tex.outputs["Color"], bsdf.inputs["Base Color"])
    return mat


# ---------------------------------------------------------------------------
# Geometry. Every solid part is a box (or the ramp's wedge) in one of a few
# meshes, one per material, with texture coordinates taken from where each
# face is in the world, so the stone runs on across boxes without seams.


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

    def wedge(self, x0, x1, z0, z1, top):
        """A ramp across x0 to x1, rising from the floor at z0 to top at z1."""
        bm = self.bm
        v = [bm.verts.new(blender(*p)) for p in (
            (x0, 0, z0), (x1, 0, z0), (x1, 0, z1), (x0, 0, z1), (x0, top, z1), (x1, top, z1))]
        for face in ((0, 3, 2, 1), (0, 1, 5, 4), (3, 4, 5, 2), (0, 4, 3), (1, 2, 5)):
            bm.faces.new([v[i] for i in face])

    def finish(self, mat, collide=True):
        bm = self.bm
        bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
        uv = bm.loops.layers.uv.new("UVMap")
        for face in bm.faces:
            n = face.normal
            for loop in face.loops:
                co = loop.vert.co
                if abs(n.z) >= abs(n.x) and abs(n.z) >= abs(n.y):
                    loop[uv].uv = (co.x / TILE, co.y / TILE)
                elif abs(n.x) >= abs(n.y):
                    loop[uv].uv = (co.y / TILE, co.z / TILE)
                else:
                    loop[uv].uv = (co.x / TILE, co.z / TILE)
        mesh = bpy.data.meshes.new(self.name)
        bm.to_mesh(mesh)
        bm.free()
        obj = bpy.data.objects.new(self.name, mesh)
        obj.data.materials.append(mat)
        if not collide:
            obj["collide"] = False
        bpy.context.scene.collection.objects.link(obj)
        return obj


def walls_around(walls, room, openings=(), sides="nsew"):
    """Walls round a room, outside it, from below its floor to its top, with
    openings: (side, from, to, top), side n, s, e or w, from and to along
    the wall (x for n and s, z for e and w), top the opening's height above
    the floor; the wall above it is a lintel."""
    x0, x1, z0, z1, floor, top = room
    bottom = 0 if floor == 0 else floor
    spans = {
        "s": (x0 - T, x1 + T, z0 - T, z0),
        "n": (x0 - T, x1 + T, z1, z1 + T),
        "w": (x0 - T, x0, z0, z1),
        "e": (x1, x1 + T, z0, z1),
    }
    for side in sides:
        ax0, ax1, az0, az1 = spans[side]
        cuts = sorted((a, b, h) for s, a, b, h in openings if s == side)
        along_x = side in "ns"
        start = ax0 if along_x else az0
        end = ax1 if along_x else az1
        pieces = []
        at = start
        for a, b, h in cuts:
            pieces.append((at, a, top))
            pieces.append((a, b, None if h is None else (floor + h, top)))
            at = b
        pieces.append((at, end, top))
        for a, b, height in pieces:
            if height is None:
                continue
            ylow, yhigh = (bottom, top) if not isinstance(height, tuple) else height
            if along_x:
                walls.box(a, b, ylow, yhigh, az0, az1)
            else:
                walls.box(ax0, ax1, ylow, yhigh, a, b)


def build_level(images):
    stone = material("Stone", images["wall"])
    flags = material("Flagstones", images["floor"])
    iron = material("Iron", color=(0.08, 0.075, 0.07, 1), roughness=0.5, metallic=1.0)

    walls = Builder("Walls")
    floors = Builder("Floors")
    brackets = Builder("TorchBrackets")

    # Floors: a slab under each room on the ground, its top at 0.
    for room in (ROOM_A, CORRIDOR_1, ROOM_B, CORRIDOR_2, ROOM_C):
        x0, x1, z0, z1, _, _ = room
        floors.box(x0 - T, x1 + T, -0.5, 0, z0 - T, z1 + T)
    # The raised floors stand on stone from the ground up.
    floors.box(ROOM_C[0], ROOM_C[1], 0, UPPER, UPPER_Z, ROOM_C[3])
    floors.box(CORRIDOR_3[0] - T, CORRIDOR_3[1] + T, 0, UPPER, CORRIDOR_3[2], CORRIDOR_3[3])
    floors.box(ROOM_D[0] - T, ROOM_D[1] + T, 0, UPPER, ROOM_D[2] - T, ROOM_D[3] + T)
    floors.box(CORRIDOR_3[0], CORRIDOR_3[1], 0, UPPER, ROOM_C[3], ROOM_C[3] + T)  # under the third doorway

    # Walls, with the doorways. Door heights are above each room's floor.
    walls_around(walls, ROOM_A, [("n", -1.2, 1.2, DOOR_HEIGHT)])
    walls_around(walls, CORRIDOR_1, sides="ew")
    walls_around(walls, ROOM_B, [("s", -1.2, 1.2, DOOR_HEIGHT), ("e", 14.7, 16.7, DOOR_HEIGHT)])
    walls_around(walls, CORRIDOR_2, sides="ns")
    walls_around(walls, ROOM_C, [("w", 14.7, 16.7, DOOR_HEIGHT), ("n", 17.0, 19.4, UPPER + DOOR_HEIGHT)])
    walls_around(walls, CORRIDOR_3, sides="ew")
    walls_around(walls, ROOM_D, [("s", 17.0, 19.4, DOOR_HEIGHT + 0.9)])

    # The great hall: stairs up the west side, the gallery along the north
    # wall, a parapet along its edge, and the ramp down the east side.
    rise = GALLERY_TOP / STEPS
    depth = (GALLERY_Z - STAIRS_Z0) / STEPS
    for i in range(STEPS):
        walls.box(STAIRS_X[0], STAIRS_X[1], 0, rise * (i + 1), STAIRS_Z0 + depth * i, STAIRS_Z0 + depth * (i + 1))
    walls.box(ROOM_B[0], ROOM_B[1], GALLERY_TOP - GALLERY_SLAB, GALLERY_TOP, GALLERY_Z, ROOM_B[3])
    walls.box(STAIRS_X[1], RAMP_X[0], GALLERY_TOP, GALLERY_TOP + PARAPET, GALLERY_Z, GALLERY_Z + 0.25)
    walls.wedge(RAMP_X[0], RAMP_X[1], RAMP_Z0, GALLERY_Z, GALLERY_TOP)

    # The lift hall: a parapet along the upper floor's edge, open where the
    # lift arrives.
    lift_at, lift_size = LIFT
    lx0, lx1 = lift_at[0] - lift_size[0] / 2, lift_at[0] + lift_size[0] / 2
    walls.box(ROOM_C[0], lx0, UPPER, UPPER + PARAPET, UPPER_Z, UPPER_Z + 0.25)
    walls.box(lx1, ROOM_C[1], UPPER, UPPER + PARAPET, UPPER_Z, UPPER_Z + 0.25)

    for x, z, size, height in PILLARS:
        floor = 0
        walls.box(x - size / 2, x + size / 2, floor, height, z - size / 2, z + size / 2)
        # A wider foot and head, so the pillars look built.
        walls.box(x - size / 2 - 0.1, x + size / 2 + 0.1, floor, floor + PLINTH, z - size / 2 - 0.1, z + size / 2 + 0.1)
        if height > 3:
            walls.box(x - size / 2 - 0.1, x + size / 2 + 0.1, height - 0.3, height, z - size / 2 - 0.1, z + size / 2 + 0.1)

    for x, z, width, length in COFFINS:
        walls.box(x - width / 2, x + width / 2, UPPER, UPPER + 0.7, z - length / 2, z + length / 2)
        walls.box(x - width / 2 - 0.08, x + width / 2 + 0.08, UPPER + 0.7, UPPER + 0.9, z - length / 2 - 0.08, z + length / 2 + 0.08)

    # Torch brackets: an iron plate on the wall and an arm out to the flame.
    for (x, up, z), facing in TORCHES:
        fx, fz = FACING[facing]
        wall_x, wall_z = x, z
        side = 0.12
        plate = (0.05 if fx else side, 0.05 if fz else side)
        brackets.box(wall_x - plate[0] + fx * 0.0, wall_x + plate[0] + fx * 0.05, up - 0.45, up - 0.1,
                     wall_z - plate[1], wall_z + plate[1] + fz * 0.05)
        ax, az = x + fx * 0.2, z + fz * 0.2
        brackets.box(min(x, ax) - 0.03, max(x, ax) + 0.03, up - 0.3, up - 0.24, min(z, az) - 0.03, max(z, az) + 0.03)
        brackets.box(ax - 0.05, ax + 0.05, up - 0.3, up - 0.05, az - 0.05, az + 0.05)

    # Iron posts at the lift's corners, from the floor to above the upper
    # floor, where it runs.
    rails = Builder("LiftPosts")
    post = 0.14
    for cx in (lx0 - post / 2, lx1 + post / 2):
        for cz in (lift_at[2] - lift_size[2] / 2 - post / 2, lift_at[2] + lift_size[2] / 2 + post / 2):
            rails.box(cx - post / 2, cx + post / 2, 0, UPPER + 0.9, cz - post / 2, cz + post / 2)

    walls.finish(stone)
    floors.finish(flags)
    rails.finish(iron)
    brackets.finish(iron, collide=False)


# ---------------------------------------------------------------------------
# Markers.


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
    for room, position, yaw in CHECKPOINTS:
        empty("Checkpoint." + room.replace(" ", "-"), "checkpoint", position, yaw, 'SINGLE_ARROW', 0.6, room=room)
    for name, position, size in DOORS:
        # A cube empty of size 0.5 scaled to the door is the door itself in
        # Blender's viewport.
        empty(name, "door", position, 0, 'CUBE', 0.5, scale=size)
    for name, (x, z), floor, key in CRATES:
        empty(name, "crate", (x, floor + CRATE / 2, z), 0, 'CUBE', 0.5, scale=(CRATE, CRATE, CRATE), key=1 if key else 0)
    for name, position in KEYS:
        empty(name, "key", position, 0, 'CIRCLE', 0.2)
    lift_at, lift_size = LIFT
    empty("Lift", "lift", lift_at, 0, 'CUBE', 0.5, scale=lift_size, rise=LIFT_RISE)
    for name, route, points in GUARDS:
        empty(name, "guard", points[0], 0, 'SINGLE_ARROW', 0.6, route=route)
        for i, p in enumerate(points):
            empty("Patrol.%s.%d" % (route, i + 1), "patrol", p, 0, 'SPHERE', 0.2, route=route, order=i + 1)
    for i, ((x, up, z), facing) in enumerate(TORCHES):
        fx, fz = FACING[facing]
        empty("Torch.%02d" % (i + 1), "torch", (x + fx * 0.2, up, z + fz * 0.2), 0, 'CONE', 0.15)


def main():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.context.preferences.filepaths.save_version = 0  # no level.blend1 backups
    bpy.context.scene.name = "Crypt"
    images = textures()
    for image in images.values():
        image.pack()  # the .blend carries its textures
    build_level(images)
    place_markers()
    bpy.ops.wm.save_as_mainfile(filepath=BLEND)
    bpy.ops.export_scene.gltf(
        filepath=GLB,
        export_format='GLB',
        export_extras=True,     # the markers' custom properties, and collide = false
        export_yup=True,        # +Y up, as GoLib expects
        use_active_scene=True,
        export_animations=False,
    )
    print("saved", BLEND, "and", GLB)


main()
