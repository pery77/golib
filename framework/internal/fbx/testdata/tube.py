# Builds a tube bent by three bones with smooth weights, a shape key, Bulge,
# and two actions, Sway (bones) and Pulse (the shape key), and exports it
# beside this script as tube.glb and tube.fbx: the test data of package fbx
# and of golib's TestFBXMatchesGLTF. From the project's folder:
#
#   Blender --background --python framework/internal/fbx/testdata/tube.py
import math
import os

import bpy

here = os.path.dirname(os.path.abspath(__file__))
bpy.ops.wm.read_factory_settings(use_empty=True)
scene = bpy.context.scene
scene.render.fps = 30

rings, around = 13, 24
verts, faces = [], []
for r in range(rings):
    for k in range(around):
        a = 2 * math.pi * k / around
        verts.append((0.25 * math.cos(a), 0.25 * math.sin(a), 3 * r / (rings - 1)))
for r in range(rings - 1):
    for k in range(around):
        a, b = r * around + k, r * around + (k + 1) % around
        faces.append((a, b, b + around, a + around))
faces.append(tuple(reversed(range(around))))
faces.append(tuple(range((rings - 1) * around, rings * around)))
data = bpy.data.meshes.new("Tube")
data.from_pydata(verts, [], faces)
tube = bpy.data.objects.new("Tube", data)
scene.collection.objects.link(tube)
bpy.context.view_layer.objects.active = tube
tube.select_set(True)
bpy.ops.object.shade_smooth()
material = bpy.data.materials.new("Orange")
material.diffuse_color = (1.0, 0.4, 0.1, 1)
material.use_nodes = True
material.node_tree.nodes["Principled BSDF"].inputs["Base Color"].default_value = (1.0, 0.35, 0.05, 1)
tube.data.materials.append(material)

bpy.ops.object.armature_add(location=(0, 0, 0))
rig = bpy.context.active_object
rig.name = "Rig"
bpy.ops.object.mode_set(mode='EDIT')
bones = rig.data.edit_bones
first = bones[0]
first.name = "Base"
first.head, first.tail = (0, 0, 0), (0, 0, 1)
middle = bones.new("Middle")
middle.head, middle.tail, middle.parent, middle.use_connect = (0, 0, 1), (0, 0, 2), first, True
top = bones.new("Top")
top.head, top.tail, top.parent, top.use_connect = (0, 0, 2), (0, 0, 3), middle, True
bpy.ops.object.mode_set(mode='OBJECT')

tube.select_set(True)
rig.select_set(True)
bpy.context.view_layer.objects.active = rig
bpy.ops.object.parent_set(type='ARMATURE_AUTO')

tube.shape_key_add(name="Basis")
bulge = tube.shape_key_add(name="Bulge")
for v in tube.data.vertices:
    x, y, z = v.co
    grow = 1 + 0.8 * math.exp(-((z - 1.5) ** 2) / 0.2)
    bulge.data[v.index].co = (x * grow, y * grow, z)

rig.animation_data_create()
sway = bpy.data.actions.new("Sway")
rig.animation_data.action = sway
for frame, angle in ((1, 0), (15, 35), (30, 0), (45, -35), (60, 0)):
    for name in ("Middle", "Top"):
        bone = rig.pose.bones[name]
        bone.rotation_mode = 'XYZ'
        bone.rotation_euler = (0, 0, math.radians(angle))
        bone.keyframe_insert("rotation_euler", frame=frame)
track = rig.animation_data.nla_tracks.new()
track.strips.new("Sway", 1, sway)
rig.animation_data.action = None

keys = tube.data.shape_keys
keys.animation_data_create()
pulse = bpy.data.actions.new("Pulse")
keys.animation_data.action = pulse
for frame, value in ((1, 0), (15, 1), (30, 0)):
    bulge.value = value
    bulge.keyframe_insert("value", frame=frame)
track = keys.animation_data.nla_tracks.new()
track.strips.new("Pulse", 1, pulse)
keys.animation_data.action = None

scene.frame_start, scene.frame_end = 1, 60
bpy.ops.export_scene.gltf(filepath=os.path.join(here, "tube.glb"),
                          export_format='GLB', export_yup=True, export_animations=True,
                          export_animation_mode='ACTIONS', export_morph=True)

# The same scene as FBX, as Blender's exporter writes it by default, with
# each action as a take.
bpy.ops.export_scene.fbx(filepath=os.path.join(here, "tube.fbx"),
                         bake_anim_use_all_actions=True, bake_anim_use_nla_strips=False)
