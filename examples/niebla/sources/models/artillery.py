import math
import sys
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))

from studio import (
    beam, box, cylinder, hull, init_scene, material, render_shadow_sheet,
    render_sheet, root, save_blend,
)


def create_scene():
    init_scene()
    armor = material("Weathered red armor", (0.55, 0.16, 0.11), 0.25)
    side = material("Dark armored sides", (0.22, 0.07, 0.06), 0.3)
    top = material("Sunlit red panels", (0.73, 0.25, 0.16), 0.18)
    rubber = material("Tire rubber", (0.025, 0.029, 0.035), 0.0, 0.94)
    hub_metal = material("Dark wheel hubs", (0.08, 0.09, 0.10), 0.5)
    steel = material("Barrel steel", (0.27, 0.30, 0.31), 0.7, 0.45)
    bore = material("Dark bore", (0.012, 0.013, 0.018), 0.0)
    brass = material("Amber lamps", (1.0, 0.52, 0.22), 0.2, 0.28)

    owner = root("Artillery")
    base = [
        (8.5, 0), (6.5, 3.3), (-6.2, 3.3), (-8.2, 2.4),
        (-8.2, -2.4), (-6.2, -3.3), (6.5, -3.3),
    ]
    hull("Armored chassis", owner, base, 2.7, 8.5, top, side)
    box("Lower frame", owner, (-0.5, 0, 3.7), (15, 6.6, 2), side, 0.45)
    box("Upper decking", owner, (-0.6, 0, 8.45),
        (11, 5.0, 0.55), armor, 0.35)

    for direction in (-1, 1):
        for index, x in enumerate((-5.5, -0.1, 5.2)):
            wheel = cylinder(f"Tire {direction} {index}", owner,
                             (x, direction * 4.0, 2.2), 2.2, 1.2, rubber)
            wheel.rotation_euler.x = math.pi / 2
            hub = cylinder(f"Hub {direction} {index}", owner,
                           (x, direction * 4.67, 2.2), 0.62, 0.16,
                           hub_metal)
            hub.rotation_euler.x = math.pi / 2
        box(f"Wheel arch {direction}", owner,
            (-0.2, direction * 3.65, 5.0),
            (15.5, 0.6, 0.7), side, 0.2)

    turret = [
        (4.3, 0), (3.0, 2.5), (-2.6, 2.5), (-3.5, 1.5),
        (-3.5, -1.5), (-2.6, -2.5), (3.0, -2.5),
    ]
    hull("Rotating gun housing", owner, turret, 8.6, 13.2, armor, side)
    box("Turret roof", owner, (-0.2, 0, 13.15),
        (4.8, 3.4, 0.5), top, 0.25)
    box("Recoil sleeve", owner, (3.5, 0, 12.8),
        (4.0, 3.0, 2.6), steel, 0.35)
    beam("Gun jacket", owner, (3.7, 0, 13.0),
         (7.5, 0, 16.5), 1.45, side)
    beam("Raised cannon", owner, (6.6, 0, 15.6),
         (15.6, 0, 23.2), 0.85, steel)
    beam("Muzzle brake", owner, (14.9, 0, 22.6),
         (16.2, 0, 23.7), 1.15, side)
    beam("Bore", owner, (16.18, 0, 23.68),
         (16.25, 0, 23.75), 0.55, bore)

    box("Front intake", owner, (8.1, 0, 5.1),
        (0.35, 3.0, 1.2), steel, 0.1)
    for direction in (-1, 1):
        box(f"Headlamp {direction}", owner,
            (7.6, direction * 2.25, 6.35),
            (0.6, 0.95, 0.7), brass, 0.12)
        box(f"Rear vent {direction}", owner,
            (-5.8, direction * 2.25, 8.8),
            (1.5, 0.9, 0.3), steel, 0.08)

    save_blend(owner, "artillery")


if __name__ == "__main__":
    if "--shadows-only" in sys.argv:
        render_shadow_sheet("Artillery", "rival-artillery")
    else:
        if "--create" in sys.argv:
            create_scene()
        render_sheet("Artillery", "rival-artillery")
        render_shadow_sheet("Artillery", "rival-artillery")
