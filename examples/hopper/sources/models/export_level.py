# Exports the Level scene of level.blend to the game's assets/models/level.glb.
#
# Run it from Blender's Scripting tab (the text "export_level.py" in
# level.blend, then Run Script), or from a terminal, from the GoLib folder:
#
#   Blender --background examples/hopper/sources/models/level.blend --python examples/hopper/sources/models/export_level.py
#
# (on macOS, Blender is /Applications/Blender.app/Contents/MacOS/Blender).
#
# Before exporting, it marks every instance of a solid kit model (the
# "block-*" collections, the trees and the rocks) as a solid block for the
# game: the custom property kind = "block", and model = the collection's
# name, which is the kit model the game measures for the block's size. An
# instance that already has another kind, such as "scenery", is left alone
# and isn't solid.

import os

import bpy

LEVEL_SCENE = "Level"
SOLID = ("block-", "tree", "rocks")  # kit collections whose instances are solid


def mark_blocks(scene):
    solid = 0
    for obj in scene.objects:
        collection = obj.instance_collection if obj.instance_type == 'COLLECTION' else None
        if collection is None or not collection.name.startswith(SOLID):
            continue
        if "kind" not in obj:
            obj["kind"] = "block"
        if obj["kind"] == "block":
            obj["model"] = collection.name
            solid += 1
    return solid


def export(out_path):
    scene = bpy.data.scenes[LEVEL_SCENE]
    if bpy.context.window is not None:
        bpy.context.window.scene = scene
    solid = mark_blocks(scene)
    with bpy.context.temp_override(scene=scene):
        bpy.ops.export_scene.gltf(
            filepath=out_path,
            export_format='GLB',
            export_extras=True,       # the custom properties: kind, model
            export_yup=True,          # +Y up, as GoLib expects
            use_active_scene=True,    # the Level scene only, not the Kit
            use_selection=False,
        )
    print("exported %d solid blocks to %s" % (solid, out_path))


def game_folder():
    blend = bpy.data.filepath
    if not blend:
        raise SystemExit("Save level.blend first: the export goes next to it, in the game's assets folder.")
    # examples/hopper/sources/models/level.blend -> examples/hopper
    return os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(blend))))


if __name__ == "__main__":
    export(os.path.join(game_folder(), "assets", "models", "level.glb"))
