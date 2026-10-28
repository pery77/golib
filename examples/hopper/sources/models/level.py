# Builds Hopper's level in Blender from Kenney's Platformer Kit (CC0), saves
# it as level.blend next to this file and exports it to the game's
# assets/models/level.glb.
#
# Run it from the GoLib folder, with Blender in the background:
#
#   Blender --background --factory-startup --python examples/hopper/sources/models/level.py
#
# (on macOS, Blender is /Applications/Blender.app/Contents/MacOS/Blender).
#
# Run it only to start the level over: it overwrites level.blend. To change
# the level, open level.blend in Blender, move things around, and run the
# export_level.py text in its Scripting tab (see DESIGN.md, "The level in
# Blender").
#
# The level is made of:
#   - Blocks: instances of the kit's models, which the Kit scene holds, one
#     collection per model. Blocks and trees are solid; export_level.py marks
#     them with the custom properties kind = "block" and model = <kit model>.
#     Instances with kind = "scenery" are only to look at.
#   - The goal: an instance of the flag, with kind = "goal".
#   - Plain empties: "Start", where the character starts (kind = "start"),
#     coins (kind = "coin") and checkpoints (kind = "checkpoint"), which the
#     game draws itself, since it hides coins once they are picked up.
#
# Positions below are written as the game sees them: (x, ahead, up), in
# meters, where ahead is the way the level runs (the game's +Z, Blender's -Y)
# and up is Blender's +Z. A block's position is the middle of its bottom.

import os
import sys

import bpy

HERE = os.path.dirname(os.path.abspath(__file__))
GAME = os.path.dirname(os.path.dirname(HERE))
KIT = os.path.join(GAME, "assets", "models", "kit")
BLEND = os.path.join(HERE, "level.blend")
GLB = os.path.join(GAME, "assets", "models", "level.glb")

sys.dont_write_bytecode = True  # no __pycache__ next to the script
sys.path.insert(0, HERE)
import export_level  # noqa: E402  (next to this file)

# The kit models the level uses; each becomes a collection in the Kit scene.
# They are all in assets/models/kit/, where the game measures the solid ones.
# To use another of the kit's models, copy its .glb there and add it here.
KIT_MODELS = [
    "block-grass", "block-grass-large", "block-grass-low", "block-grass-low-long",
    "block-grass-narrow", "block-grass-large-tall", "block-grass-long",
    "block-grass-low-large", "block-grass-hexagon",
    "tree", "tree-pine", "flowers", "grass", "rocks", "flag",
]

# Solid things: (kit model, x, ahead, up, yaw in degrees).
BLOCKS = [
    # The start: two large blocks.
    ("block-grass-large", 0, 0, 0, 0),
    ("block-grass-large", 0, 2.08, 0, 0),
    # Three hops, each a little higher.
    ("block-grass", 0, 4.8, 0, 0),
    ("block-grass", -0.6, 7.0, 0.5, 0),
    ("block-grass", 0.4, 9.3, 1.0, 0),
    # A rest island, with a block to the side for a bonus coin.
    ("block-grass-large", 0, 12.4, 1.0, 0),
    ("block-grass", 3.2, 11.8, 1.5, 0),
    ("tree", 0.5, 12.95, 2.0, 0),
    # A zigzag over narrow blocks.
    ("block-grass-narrow", -1.6, 15.2, 1.0, 0),
    ("block-grass-narrow", 0.2, 17.2, 1.5, 0),
    ("block-grass-narrow", 2.0, 19.2, 2.0, 0),
    # The checkpoint island.
    ("block-grass-large", 1.2, 22.2, 2.0, 0),
    ("tree-pine", 2.0, 23.0, 3.0, 0),
    # A bridge of low blocks, with gaps.
    ("block-grass-low-long", 1.2, 26.0, 2.5, 90),
    ("block-grass-low-long", -0.8, 29.4, 2.5, 90),
    ("block-grass-low", -0.8, 32.2, 3.0, 0),
    # The climb.
    ("block-grass", 0.6, 34.4, 3.3, 0),
    ("block-grass-large-tall", -0.4, 37.6, 2.5, 0),
    # The long jump to the goal.
    ("block-grass-large", -0.4, 41.8, 3.5, 0),
    ("block-grass-large", -0.4, 43.88, 3.5, 0),
    ("rocks", 0.5, 44.4, 4.5, 30),
]

# Only to look at: (kit model, x, ahead, up, yaw). Far islands under the
# level, and plants, which the character walks through.
SCENERY = [
    ("block-grass-large-tall", -9, 6, -4, 0),
    ("tree-pine", -9.4, 6.4, -2, 0),
    ("tree", -8.5, 5.5, -2, 40),
    ("block-grass-large-tall", 9, 17, -5, 0),
    ("tree", 9, 17, -3, 0),
    ("block-grass-large-tall", -10, 29, -3, 0),
    ("tree-pine", -10, 29, -1, 0),
    ("block-grass-large-tall", 10, 39, -2, 0),
    ("tree-pine", 10.4, 38.6, 0, 0),
    ("tree", 9.5, 39.5, 0, 0),
    ("block-grass-large-tall", -7, 50, 0, 0),
    ("flowers", 0.5, 0.7, 1, 0),
    ("grass", -0.6, 1.9, 1, 0),
    ("grass", -0.7, 13.0, 2, 30),
    ("flowers", 0.6, 22.8, 3, 20),
    ("grass", -0.8, 37.0, 4.5, 0),
    ("flowers", -1.0, 43.6, 4.5, 0),
]

START = (0, 0, 1)            # (x, ahead, up): the character's feet
GOAL = (-0.4, 44.2, 4.5)     # the flag, at the end
GOAL_SCALE = 2.0             # the kit's flag is 0.9 m tall: make it stand out
CHECKPOINTS = [(0.4, 22.6, 3)]

# Coins: (x, ahead, up) of their middle. Over the blocks, and on the arcs of
# the jumps between them.
COINS = [
    (0, 3.9, 2.2), (0, 4.8, 1.6),
    (-0.3, 5.9, 2.6), (-0.6, 7.0, 2.1),
    (-0.1, 8.15, 3.1), (0.4, 9.3, 2.6),
    (-0.5, 11.8, 2.6), (0.4, 11.8, 2.6), (3.2, 11.8, 3.1),
    (-1.6, 15.2, 2.6), (0.2, 17.2, 3.1), (2.0, 19.2, 3.6),
    (1.2, 25.4, 3.6), (1.2, 26.6, 3.6),
    (0.2, 27.9, 4.2), (-0.8, 29.4, 3.6),
    (-0.8, 32.2, 4.1), (0.6, 34.4, 4.9),
    (-0.4, 37.6, 5.1), (-0.4, 39.7, 5.6),
]


def blender_location(x, ahead, up):
    """The game's (x, ahead, up) in Blender's axes: ahead is Blender's -Y."""
    return (x, -ahead, up)


def import_kit(kit_scene):
    """Imports each kit model into its own collection of the Kit scene, at
    the origin, which is where an instance places it. (Don't spread them out
    with the collections' instance offset: Blender's glTF export moves the
    instancing empty by the offset, and the game reads the empty's place.)
    Every model shares the kit's one material and texture."""
    bpy.context.window.scene = kit_scene
    collections = {}
    shared = None
    for name in KIT_MODELS:
        before = set(bpy.data.objects)
        bpy.ops.import_scene.gltf(filepath=os.path.join(KIT, name + ".glb"))
        new = [o for o in bpy.data.objects if o not in before]
        collection = bpy.data.collections.new(name)
        kit_scene.collection.children.link(collection)
        for obj in new:
            for c in list(obj.users_collection):
                c.objects.unlink(obj)
            collection.objects.link(obj)
            for slot in obj.material_slots:
                if shared is None:
                    shared = slot.material
                elif slot.material is not shared:
                    slot.material = shared
        collections[name] = collection
    for mat in list(bpy.data.materials):
        if mat is not shared and mat.users == 0:
            bpy.data.materials.remove(mat)
    for image in list(bpy.data.images):
        if image.users == 0:
            bpy.data.images.remove(image)
    return collections


def instance(scene, collections, model, x, ahead, up, yaw, kind=None, name=None, scale=1.0):
    obj = bpy.data.objects.new(name or model, None)
    obj.instance_type = 'COLLECTION'
    obj.instance_collection = collections[model]
    obj.location = blender_location(x, ahead, up)
    obj.rotation_euler = (0, 0, yaw * 3.141592653589793 / 180)
    obj.scale = (scale, scale, scale)
    if kind:
        obj["kind"] = kind
    scene.collection.objects.link(obj)
    return obj


def empty(scene, name, kind, position, display, size):
    obj = bpy.data.objects.new(name, None)
    obj.empty_display_type = display
    obj.empty_display_size = size
    obj.location = blender_location(*position)
    obj["kind"] = kind
    scene.collection.objects.link(obj)
    return obj


def main():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.context.preferences.filepaths.save_version = 0  # no level.blend1 backups
    level = bpy.context.scene
    level.name = export_level.LEVEL_SCENE
    kit = bpy.data.scenes.new("Kit")
    collections = import_kit(kit)
    bpy.context.window.scene = level

    for model, x, ahead, up, yaw in BLOCKS:
        instance(level, collections, model, x, ahead, up, yaw)
    for model, x, ahead, up, yaw in SCENERY:
        instance(level, collections, model, x, ahead, up, yaw, kind="scenery")
    instance(level, collections, "flag", *GOAL, 0, kind="goal", name="Goal", scale=GOAL_SCALE)
    empty(level, "Start", "start", START, 'SINGLE_ARROW', 1.0)
    for i, at in enumerate(CHECKPOINTS):
        empty(level, "Checkpoint.%03d" % (i + 1), "checkpoint", at, 'CONE', 0.4)
    for i, at in enumerate(COINS):
        coin = empty(level, "Coin.%03d" % (i + 1), "coin", at, 'CIRCLE', 0.2)
        coin.rotation_euler = (3.141592653589793 / 2, 0, 0)  # stood up, as the game draws it

    # The export script travels inside the .blend, for its Scripting tab.
    with open(os.path.join(HERE, "export_level.py")) as f:
        bpy.data.texts.new("export_level.py").from_string(f.read())

    bpy.ops.wm.save_as_mainfile(filepath=BLEND)
    export_level.export(GLB)
    bpy.ops.wm.save_as_mainfile(filepath=BLEND)  # with the properties the export set
    print("saved", BLEND)


main()
