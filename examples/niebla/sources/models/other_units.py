import sys
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))

from studio import (
    beam, box, cylinder, hull, init_scene, loaded_model_name, material,
    render_shadow_sheet, render_sheet, root, save_blend, wheels,
)


def palette(name, body, top, dark, glow):
    return {
        "body": material(name + " armor", body, 0.22),
        "top": material(name + " highlights", top, 0.14),
        "dark": material(name + " undercarriage", dark, 0.3),
        "glow": material(name + " lamps", glow, 0.1, 0.25),
        "rubber": material(name + " tires", (0.025, 0.029, 0.035),
                           0.0, 0.94),
        "hub": material(name + " wheel hubs", (0.08, 0.09, 0.10), 0.5),
        "steel": material(name + " steel", (0.27, 0.30, 0.31),
                          0.65, 0.45),
    }


def worker_core():
    p = palette("Core worker", (0.75, 0.78, 0.69),
                (0.93, 0.91, 0.72), (0.23, 0.27, 0.27),
                (1.0, 0.76, 0.35))
    unit = root("Model")
    body = [
        (4.1, 0), (2.7, 1.65), (-1.9, 1.75), (-3.2, 1.2),
        (-2.8, 0), (-3.2, -1.2), (-1.9, -1.75), (2.7, -1.65),
    ]
    hull("Narrow scouting hull", unit, body, 1.1, 4.15,
         p["top"], p["dark"])
    box("Center spine", unit, (0.0, 0, 4.2),
        (4.4, 1.15, 0.6), p["body"], 0.2)
    wheels(unit, (-2.1, 1.8), 2.0, 0.87,
           p["rubber"], p["hub"])
    box("Forward sensor", unit, (3.3, 0, 4.25),
        (0.65, 0.9, 0.8), p["glow"], 0.16)
    for side in (-1, 1):
        box(f"Rear lamp {side}", unit, (-2.7, side * 1.13, 3.65),
            (0.5, 0.45, 0.6), p["glow"], 0.12)
    return unit


def worker_carrier():
    p = palette("Teal carrier", (0.08, 0.42, 0.45),
                (0.14, 0.59, 0.59), (0.055, 0.18, 0.20),
                (1.0, 0.63, 0.19))
    unit = root("Model")
    body = [
        (4.35, 0), (3.5, 2.3), (-3.4, 2.4), (-4.2, 1.5),
        (-4.2, -1.5), (-3.4, -2.4), (3.5, -2.3),
    ]
    hull("Carrier cargo frame", unit, body, 1.3, 4.7,
         p["top"], p["dark"])
    wheels(unit, (-2.9, 2.75), 2.8, 1.08,
           p["rubber"], p["hub"])
    box("Cargo platform", unit, (-1.15, 0, 4.85),
        (4.3, 3.3, 0.35), p["dark"], 0.15)
    tank = cylinder("Visible oil tank", unit,
                    (-1.7, 0, 6.4), 1.25, 2.8,
                    p["steel"], 12)
    tank.rotation_euler.x = 1.57
    for side in (-1, 1):
        box(f"Cargo rail {side}", unit,
            (-0.8, side * 2.2, 5.65),
            (5.3, 0.34, 1.1), p["body"], 0.12)
        box(f"Front lamp {side}", unit,
            (3.7, side * 1.45, 3.75),
            (0.5, 0.55, 0.6), p["glow"], 0.1)
    box("Tank sight glass", unit, (-1.5, -1.54, 6.3),
        (1.1, 0.22, 0.65), p["glow"], 0.12)
    return unit


def worker_trooper():
    p = palette("Olive trooper", (0.32, 0.43, 0.19),
                (0.49, 0.56, 0.29), (0.12, 0.19, 0.11),
                (0.93, 0.85, 0.44))
    unit = root("Model")
    body = [
        (4.15, 0), (3.1, 2.8), (-3.1, 2.8), (-4.1, 1.7),
        (-4.1, -1.7), (-3.1, -2.8), (3.1, -2.8),
    ]
    hull("Wide armored hull", unit, body, 1.6, 5.0,
         p["top"], p["dark"])
    wheels(unit, (-2.5, 2.4), 3.2, 1.12,
           p["rubber"], p["hub"])
    turret = [
        (2.0, 0), (1.2, 1.3), (-1.6, 1.3), (-2.1, 0),
        (-1.6, -1.3), (1.2, -1.3),
    ]
    hull("Trooper turret", unit, turret, 5.0, 7.65,
         p["body"], p["dark"])
    box("Turret cap", unit, (-0.15, 0, 7.7),
        (2.6, 1.65, 0.35), p["top"], 0.12)
    beam("Forward small arm", unit, (1.25, 0, 6.7),
         (6.4, 0, 7.5), 0.37, p["steel"])
    box("Muzzle flash window", unit, (6.3, 0, 7.6),
        (0.3, 0.42, 0.4), p["glow"], 0.09)
    for side in (-1, 1):
        box(f"Armor cheek {side}", unit,
            (-0.8, side * 2.5, 5.3),
            (3.0, 0.4, 0.8), p["body"], 0.17)
    return unit


def worker_mechanic():
    p = palette("Field mechanic", (0.38, 0.47, 0.48),
                (0.62, 0.64, 0.54), (0.16, 0.23, 0.25),
                (1.0, 0.67, 0.22))
    unit = root("Model")
    body = [
        (4.0, 0), (3.1, 2.25), (-3.25, 2.3), (-4.0, 1.45),
        (-4.0, -1.45), (-3.25, -2.3), (3.1, -2.25),
    ]
    hull("Mechanic service chassis", unit, body, 1.35, 4.7,
         p["top"], p["dark"])
    wheels(unit, (-2.55, 2.45), 2.65, 1.0, p["rubber"], p["hub"])
    box("Tool deck", unit, (-1.0, 0, 4.9),
        (4.7, 3.45, 0.4), p["body"], 0.18)
    box("Armored cab", unit, (2.05, 0, 5.4),
        (2.1, 2.65, 1.8), p["body"], 0.24)
    box("Amber service panel", unit, (-1.15, 0, 5.25),
        (2.1, 2.2, 0.38), p["glow"], 0.12)

    beam("Crane base", unit, (-2.05, 0, 5.2),
         (-2.05, 0, 7.0), 0.58, p["steel"])
    beam("Crane lower arm", unit, (-2.05, 0, 6.7),
         (-0.7, 0, 9.2), 0.48, p["body"])
    beam("Crane upper arm", unit, (-0.7, 0, 9.2),
         (-2.45, 0, 11.5), 0.38, p["steel"])
    beam("Tool cable", unit, (-2.45, 0, 11.5),
         (-2.45, 0, 10.15), 0.16, p["dark"])
    box("Repair tool", unit, (-2.45, 0, 9.85),
        (0.85, 1.15, 0.55), p["glow"], 0.14)
    for side in (-1, 1):
        box(f"Tool case {side}", unit,
            (-1.55, side * 2.0, 5.35),
            (1.7, 0.48, 0.72), p["dark"], 0.12)
        box(f"Cab lamp {side}", unit,
            (3.0, side * 1.2, 4.65),
            (0.42, 0.38, 0.52), p["glow"], 0.1)
    return unit


def rival_scout():
    p = palette("Rival scout", (0.58, 0.17, 0.13),
                (0.75, 0.28, 0.19), (0.24, 0.075, 0.055),
                (1.0, 0.38, 0.23))
    unit = root("Model")
    body = [
        (2.65, 0), (1.7, 1.35), (-1.9, 1.35), (-2.55, 0.8),
        (-2.55, -0.8), (-1.9, -1.35), (1.7, -1.35),
    ]
    hull("Rival scout shell", unit, body, 1.0, 3.75,
         p["top"], p["dark"])
    wheels(unit, (-1.5, 1.5), 1.52, 0.64,
           p["rubber"], p["hub"])
    box("Wide cyclopean eye", unit, (2.2, 0, 3.2),
        (0.42, 1.55, 0.58), p["glow"], 0.12)
    box("Sensor ridge", unit, (-0.7, 0, 4.03),
        (2.6, 0.46, 0.5), p["body"], 0.12)
    return unit


def rival_crawler():
    p = palette("Rival crawler", (0.55, 0.16, 0.11),
                (0.73, 0.25, 0.16), (0.22, 0.07, 0.06),
                (1.0, 0.52, 0.22))
    return rival_crawler_model(p)


def rival_city_crawler():
    p = palette("Rival city crawler", (0.36, 0.28, 0.28),
                (0.50, 0.40, 0.40), (0.17, 0.13, 0.14),
                (0.92, 0.48, 0.40))
    return rival_crawler_model(p)


def rival_crawler_model(p):
    unit = root("Model")
    body = [
        (8.0, 0), (6.1, 4.15), (-6.2, 4.15), (-8.1, 2.5),
        (-8.1, -2.5), (-6.2, -4.15), (6.1, -4.15),
    ]
    hull("Bulky command chassis", unit, body, 2.25, 9.0,
         p["top"], p["dark"])
    wheels(unit, (-5.1, 0, 5.15), 4.55, 1.88,
           p["rubber"], p["hub"])
    box("Command cabin", unit, (0.6, 0, 10.2),
        (7.6, 5.7, 3.0), p["body"], 0.58)
    box("Forward armored glass", unit, (4.25, 0, 10.6),
        (0.34, 3.65, 1.45), p["steel"], 0.11)
    beam("Repulsor mast", unit, (-2.8, 0, 11.0),
         (-2.8, 0, 18.0), 0.48, p["steel"])
    cylinder("Antimist emitter", unit, (-2.8, 0, 18.5),
             1.55, 1.15, p["glow"], 12)
    for side in (-1, 1):
        box(f"Front signal {side}", unit,
            (7.15, side * 2.7, 5.7),
            (0.5, 0.7, 0.8), p["glow"], 0.12)
    return unit


def rival_raider():
    p = palette("Rival raider", (0.55, 0.16, 0.11),
                (0.73, 0.25, 0.16), (0.22, 0.07, 0.06),
                (0.98, 0.48, 0.21))
    unit = root("Model")
    body = [
        (4.2, 0), (3.1, 2.35), (-3.5, 2.35), (-4.3, 1.4),
        (-4.3, -1.4), (-3.5, -2.35), (3.1, -2.35),
    ]
    hull("Raider tanker frame", unit, body, 1.4, 4.65,
         p["top"], p["dark"])
    wheels(unit, (-2.6, 2.5), 2.75, 1.03,
           p["rubber"], p["hub"])
    tank = cylinder("Stolen oil drum", unit, (-1.3, 0, 6.3),
                    1.56, 3.3, p["steel"], 12)
    tank.rotation_euler.x = 1.57
    for side in (-1, 1):
        box(f"Tank strap {side}", unit,
            (-1.3, side * 1.6, 6.3),
            (0.5, 0.3, 2.8), p["dark"], 0.1)
        box(f"Front headlamp {side}", unit,
            (3.45, side * 1.45, 3.6),
            (0.5, 0.55, 0.62), p["glow"], 0.1)
    box("Stolen oil marker", unit, (-1.3, -1.95, 6.45),
        (1.2, 0.28, 0.55), p["glow"], 0.13)
    return unit


MODELS = {
    "worker-core": worker_core,
    "worker-carrier": worker_carrier,
    "worker-trooper": worker_trooper,
    "worker-mechanic": worker_mechanic,
    "rival-scout": rival_scout,
    "rival-crawler": rival_crawler,
    "rival-city-crawler": rival_city_crawler,
    "rival-raider": rival_raider,
}


if __name__ == "__main__":
    if "--shadows-only" in sys.argv:
        name = loaded_model_name()
        if name not in MODELS:
            raise ValueError("load a unit .blend or pass -- --create")
        render_shadow_sheet("Model", name)
    elif "--create" in sys.argv:
        requested = sys.argv[sys.argv.index("--create") + 1:]
        for name in requested or MODELS.keys():
            if name not in MODELS:
                raise ValueError(f"unknown model {name!r}")
            init_scene()
            owner = MODELS[name]()
            save_blend(owner, name)
            render_sheet("Model", name)
            render_shadow_sheet("Model", name)
    else:
        name = loaded_model_name()
        if name not in MODELS:
            raise ValueError("load a unit .blend or pass -- --create")
        render_sheet("Model", name)
        render_shadow_sheet("Model", name)
