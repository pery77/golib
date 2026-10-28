import math
import tempfile
from array import array
from pathlib import Path

import bpy
from mathutils import Vector


GAME = Path(__file__).resolve().parents[2]
FRAME_WIDTH = 128
FRAME_HEIGHT = 192
FOOT_X = 64
FOOT_Y = 160
REFERENCE_ZOOM = 32
UNIT_W = 48 / 200
UNIT_H = 24 / 200
ELEVATION = math.radians(30)
VERTICAL_SCALE = math.sqrt(2 / 3)
SHADOW_SUPERSAMPLE = 4
SHADOW_PLANE_SIZE = 80
SHADOW_ALPHA_GAIN = 1.5
SHADOW_RESOLUTION_SCALE = 4
SHADOW_RENDER_SAMPLES = 64
SHADOW_LIGHT_ENERGY = 2
SHADOW_LIGHT_ANGLE = math.radians(8)
SHADOW_LIGHT_DIRECTION = (1, -30, 55)


def material(name, color, metallic=0.0, roughness=0.75):
    result = bpy.data.materials.new(name)
    result.diffuse_color = (*color, 1)
    result.use_nodes = True
    shader = result.node_tree.nodes.get("Principled BSDF")
    shader.inputs["Base Color"].default_value = (*color, 1)
    shader.inputs["Metallic"].default_value = metallic
    shader.inputs["Roughness"].default_value = roughness
    shader.inputs["Emission Color"].default_value = (*color, 1)
    shader.inputs["Emission Strength"].default_value = 0.5
    return result


def root(name):
    obj = bpy.data.objects.new(name, None)
    bpy.context.collection.objects.link(obj)
    obj.scale.z = VERTICAL_SCALE
    return obj


def parented(obj, name, owner, surface):
    obj.name = name
    obj.parent = owner
    obj.data.materials.append(surface)
    return obj


def box(name, owner, position, dimensions, surface, bevel=0.0):
    bpy.ops.mesh.primitive_cube_add(size=1)
    obj = parented(bpy.context.object, name, owner, surface)
    obj.location = position
    obj.dimensions = dimensions
    if bevel:
        edge = obj.modifiers.new("Soft edges", "BEVEL")
        edge.width = bevel
        edge.segments = 1
        obj.modifiers.new("Face normals", "WEIGHTED_NORMAL")
    return obj


def cylinder(name, owner, position, radius, depth, surface, vertices=16):
    bpy.ops.mesh.primitive_cylinder_add(
        vertices=vertices, radius=radius, depth=depth,
    )
    obj = parented(bpy.context.object, name, owner, surface)
    obj.location = position
    return obj


def beam(name, owner, start, end, radius, surface):
    direction = Vector(end) - Vector(start)
    obj = cylinder(
        name, owner, (Vector(start) + Vector(end)) / 2,
        radius, direction.length, surface, 12,
    )
    obj.rotation_euler = direction.to_track_quat("Z", "Y").to_euler()
    return obj


def hull(name, owner, outline, bottom, top, roof, side):
    count = len(outline)
    vertices = [(x, y, bottom) for x, y in outline]
    vertices += [(x * 0.93, y * 0.93, top) for x, y in outline]
    faces = [
        tuple(range(count - 1, -1, -1)),
        tuple(range(count, count * 2)),
    ]
    faces += [
        (i, (i + 1) % count, (i + 1) % count + count, i + count)
        for i in range(count)
    ]
    mesh = bpy.data.meshes.new(name)
    mesh.from_pydata(vertices, [], faces)
    mesh.materials.append(roof)
    mesh.materials.append(side)
    mesh.update()
    for polygon in mesh.polygons[2:]:
        polygon.material_index = 1
    obj = bpy.data.objects.new(name, mesh)
    bpy.context.collection.objects.link(obj)
    obj.parent = owner
    return obj


def wheels(owner, positions, side, radius, rubber, hub_metal):
    for direction in (-1, 1):
        for index, x in enumerate(positions):
            wheel = cylinder(
                f"Tire {direction} {index}", owner,
                (x, direction * side, radius), radius,
                radius * 0.55, rubber, 12,
            )
            wheel.rotation_euler.x = math.pi / 2
            hub = cylinder(
                f"Hub {direction} {index}", owner,
                (x, direction * (side + radius * 0.3), radius),
                radius * 0.29, radius * 0.08, hub_metal, 12,
            )
            hub.rotation_euler.x = math.pi / 2


def init_scene():
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete(use_global=False)
    scene = bpy.context.scene
    scene.world.use_nodes = True
    background = scene.world.node_tree.nodes.get("Background")
    background.inputs["Color"].default_value = (0.26, 0.30, 0.34, 1)
    background.inputs["Strength"].default_value = 0.65

    pixels_per_unit = REFERENCE_ZOOM * UNIT_W / math.sqrt(2)
    focus_height = (
        (FOOT_Y - FRAME_HEIGHT / 2) /
        (pixels_per_unit * math.cos(ELEVATION))
    )
    focus = Vector((0, 0, focus_height))
    camera_data = bpy.data.cameras.new("2:1 orthographic camera")
    camera_data.type = "ORTHO"
    camera_data.ortho_scale = FRAME_HEIGHT / pixels_per_unit
    camera = bpy.data.objects.new("2:1 orthographic camera", camera_data)
    bpy.context.collection.objects.link(camera)
    camera.location = focus + Vector((80, 80, 80 * math.sqrt(2 / 3)))
    camera.rotation_euler = (focus - camera.location).to_track_quat(
        "-Z", "Y",
    ).to_euler()
    scene.camera = camera

    for name, position, power, width in (
        ("Soft daylight", (1, -30, 55), 2300, 24),
        ("Warm bounce", (20, 15, 35), 550, 18),
    ):
        light_data = bpy.data.lights.new(name, "AREA")
        light_data.energy = power
        light_data.shape = "DISK"
        light_data.size = width
        light = bpy.data.objects.new(name, light_data)
        bpy.context.collection.objects.link(light)
        light.location = position
        light.rotation_euler = (-Vector(position)).to_track_quat(
            "-Z", "Y",
        ).to_euler()


def save_blend(owner, name):
    bpy.ops.object.select_all(action="DESELECT")
    bpy.context.view_layer.objects.active = owner
    owner.select_set(True)
    bpy.context.preferences.filepaths.save_version = 0
    bpy.ops.wm.save_as_mainfile(
        filepath=str(GAME / "sources/models" / (name + ".blend")),
    )


def loaded_model_name():
    return Path(bpy.data.filepath).stem


def yaw_for_screen_octant(facing):
    angle = facing * math.pi / 4
    screen_x, screen_y = math.cos(angle), math.sin(angle)
    world_x = screen_x / UNIT_W + screen_y / UNIT_H
    world_y = screen_y / UNIT_H - screen_x / UNIT_W
    return math.atan2(world_x, world_y)


def render_sheet(owner_name, sheet_name):
    scene = bpy.context.scene
    scene.render.engine = "BLENDER_EEVEE"
    scene.render.resolution_x = FRAME_WIDTH
    scene.render.resolution_y = FRAME_HEIGHT
    scene.render.resolution_percentage = 100
    scene.render.film_transparent = True
    scene.render.image_settings.file_format = "PNG"
    scene.render.image_settings.color_mode = "RGBA"
    scene.render.image_settings.color_depth = "8"
    scene.view_settings.view_transform = "Standard"
    scene.view_settings.look = "Medium High Contrast"

    owner = bpy.data.objects[owner_name]
    width = FRAME_WIDTH * 8
    sheet = bpy.data.images.new(sheet_name, width, FRAME_HEIGHT, alpha=True)
    pixels = array("f", [0]) * (width * FRAME_HEIGHT * 4)
    cache = GAME.parents[1] / "build/niebla"
    cache.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=cache) as tmp:
        for facing in range(8):
            owner.rotation_euler.z = yaw_for_screen_octant(facing)
            scene.render.filepath = str(Path(tmp) / f"face-{facing}.png")
            bpy.ops.render.render(write_still=True)
            frame = bpy.data.images.load(scene.render.filepath)
            frame_pixels = array("f", [0]) * (FRAME_WIDTH * FRAME_HEIGHT * 4)
            frame.pixels.foreach_get(frame_pixels)
            for row in range(FRAME_HEIGHT):
                start = (row * width + facing * FRAME_WIDTH) * 4
                offset = row * FRAME_WIDTH * 4
                pixels[start:start + FRAME_WIDTH * 4] = (
                    frame_pixels[offset:offset + FRAME_WIDTH * 4]
                )
            bpy.data.images.remove(frame)
    sheet.pixels.foreach_set(pixels)
    sheet.filepath_raw = str(GAME / "assets/sprites" / (sheet_name + ".png"))
    sheet.file_format = "PNG"
    sheet.save()
    print(f"Rendered {sheet.filepath_raw}")
    bpy.data.images.remove(sheet)


def render_pixels(scene, filepath):
    scene.render.filepath = str(filepath)
    bpy.ops.render.render(write_still=True)
    image = bpy.data.images.load(str(filepath), check_existing=False)
    pixels = array("f", [0]) * (
        int(image.size[0]) * int(image.size[1]) * 4
    )
    image.pixels.foreach_get(pixels)
    bpy.data.images.remove(image)
    return pixels


def render_shadow_sheet(owner_name, sheet_name):
    scene = bpy.context.scene
    scene.render.engine = "BLENDER_EEVEE"
    scene.render.resolution_x = FRAME_WIDTH * SHADOW_SUPERSAMPLE
    scene.render.resolution_y = FRAME_HEIGHT * SHADOW_SUPERSAMPLE
    scene.render.resolution_percentage = 100
    scene.render.film_transparent = False
    scene.render.image_settings.file_format = "PNG"
    scene.render.image_settings.color_mode = "RGBA"
    scene.render.image_settings.color_depth = "8"
    scene.view_settings.view_transform = "Standard"
    scene.view_settings.look = "Medium High Contrast"
    scene.eevee.use_shadows = True
    scene.eevee.shadow_resolution_scale = SHADOW_RESOLUTION_SCALE
    scene.eevee.taa_render_samples = SHADOW_RENDER_SAMPLES

    owner = bpy.data.objects[owner_name]
    source = bpy.data.images.load(
        str(GAME / "assets/sprites" / (sheet_name + ".png")),
        check_existing=False,
    )
    source_width = int(source.size[0])
    source_pixels = array("f", [0]) * (
        source_width * FRAME_HEIGHT * 4
    )
    source.pixels.foreach_get(source_pixels)

    bpy.ops.mesh.primitive_plane_add(
        size=SHADOW_PLANE_SIZE, location=(0, 0, 0),
    )
    floor = bpy.context.object
    floor.name = "Shadow render floor"
    floor_mesh = floor.data
    floor_material = bpy.data.materials.new("Shadow render floor")
    floor_material.use_nodes = True
    surface = floor_material.node_tree.nodes.get("Principled BSDF")
    surface.inputs["Base Color"].default_value = (1, 1, 1, 1)
    surface.inputs["Roughness"].default_value = 1
    floor.data.materials.append(floor_material)

    sun_data = bpy.data.lights.new("Shadow daylight", "SUN")
    sun_data.energy = SHADOW_LIGHT_ENERGY
    sun_data.angle = SHADOW_LIGHT_ANGLE
    sun = bpy.data.objects.new("Shadow daylight", sun_data)
    scene.collection.objects.link(sun)
    direction = Vector(SHADOW_LIGHT_DIRECTION)
    sun.rotation_euler = (-direction).to_track_quat("-Z", "Y").to_euler()

    width = FRAME_WIDTH * 8
    height = FRAME_HEIGHT
    pixels = array("f", [0]) * (width * height * 4)
    render_width = FRAME_WIDTH * SHADOW_SUPERSAMPLE
    cache = GAME.parents[1] / "build/niebla"
    cache.mkdir(parents=True, exist_ok=True)
    shadow_dir = GAME / "assets/sprites/shadows"
    shadow_dir.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(dir=cache) as tmp:
        for facing in range(8):
            owner.rotation_euler.z = yaw_for_screen_octant(facing)
            # Subtract paired renders, then use the model alpha to remove its
            # silhouette from the resulting shadow mask.
            sun_data.use_shadow = True
            shadowed = render_pixels(
                scene, Path(tmp) / f"shadow-{facing}.png",
            )
            sun_data.use_shadow = False
            lit = render_pixels(scene, Path(tmp) / f"lit-{facing}.png")

            for row in range(height):
                for column in range(FRAME_WIDTH):
                    source_index = (
                        (row * source_width + facing * FRAME_WIDTH + column) * 4
                    )
                    opacity = 0
                    for sub_y in range(SHADOW_SUPERSAMPLE):
                        for sub_x in range(SHADOW_SUPERSAMPLE):
                            render_index = (
                                (row * SHADOW_SUPERSAMPLE + sub_y) *
                                render_width +
                                column * SHADOW_SUPERSAMPLE + sub_x
                            ) * 4
                            red = lit[render_index] - shadowed[render_index]
                            green = lit[render_index + 1] - shadowed[
                                render_index + 1
                            ]
                            blue = lit[render_index + 2] - shadowed[
                                render_index + 2
                            ]
                            difference = max(
                                0,
                                red * 0.2126 + green * 0.7152 +
                                blue * 0.0722,
                            )
                            body_alpha = source_pixels[source_index + 3]
                            opacity += min(
                                1, difference * SHADOW_ALPHA_GAIN,
                            ) * (1 - body_alpha)
                    target_index = (
                        row * width + facing * FRAME_WIDTH + column
                    ) * 4
                    pixels[target_index] = 1
                    pixels[target_index + 1] = 1
                    pixels[target_index + 2] = 1
                    pixels[target_index + 3] = opacity / (
                        SHADOW_SUPERSAMPLE ** 2
                    )

    sheet = bpy.data.images.new(
        sheet_name + " shadow", width, height, alpha=True,
    )
    sheet.pixels.foreach_set(pixels)
    sheet.filepath_raw = str(shadow_dir / (sheet_name + ".png"))
    sheet.file_format = "PNG"
    sheet.save()
    print(f"Rendered {sheet.filepath_raw}")

    bpy.data.images.remove(sheet)
    bpy.data.images.remove(source)
    bpy.data.objects.remove(floor, do_unlink=True)
    bpy.data.objects.remove(sun, do_unlink=True)
    bpy.data.meshes.remove(floor_mesh)
    bpy.data.materials.remove(floor_material)
    bpy.data.lights.remove(sun_data)
