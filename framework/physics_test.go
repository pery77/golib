package golib

import (
	"math"
	"testing"

	"golib/internal/gltf"
)

// floorWorld is a world with a floor 20 meters a side, its top at 0.
func floorWorld() (*PhysicsWorld, int) {
	w := NewPhysicsWorld()
	floor := w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{Y: -0.5}, Scale: Vector3{X: 20, Y: 1, Z: 20}})
	return w, floor
}

func TestRaycastAndSphereCast(t *testing.T) {
	w, floor := floorWorld()
	hit, ok := w.Raycast(Vector3{X: 1, Y: 5, Z: 2}, Vector3{Y: -2}, 10)
	if !ok || !near3(hit.Position, Vector3{X: 1, Z: 2}) || !near3(hit.Normal, Vector3{Y: 1}) || !within(hit.Distance, 5, 1e-4) || hit.Body != floor {
		t.Errorf("a ray down onto the floor: %+v, %v", hit, ok)
	}
	if _, ok := w.Raycast(Vector3{Y: 5}, Vector3{Y: -1}, 4); ok {
		t.Error("a ray 4 meters long reached a floor 5 meters away")
	}
	if _, ok := w.Raycast(Vector3{Y: 5}, Vector3{Y: 1}, 100); ok {
		t.Error("a ray up hit the floor below it")
	}
	if hit, ok := w.Raycast(Vector3{Y: -5}, Vector3{Y: 1}, 10); !ok || !near3(hit.Position, Vector3{Y: -1}) {
		t.Errorf("a ray up from under the floor hits its bottom at %+v, %v, want at -1", hit, ok)
	}

	wall := w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{Z: 5, Y: 1}, Scale: Vector3{X: 4, Y: 2, Z: 1}})
	hit, ok = w.SphereCast(Vector3{Y: 1}, Vector3{Z: 1}, 0.5, 20)
	if !ok || hit.Body != wall || !within(hit.Position.Z, 4, 1e-3) || !near3(hit.Normal, Vector3{Z: -1}) {
		t.Errorf("a ball of radius 0.5 rolled at the wall's face at z = 4.5 stops at %+v, %v, want its center at z = 4", hit, ok)
	}
	// Past the wall's corner, a ball grazes its edge.
	hit, ok = w.SphereCast(Vector3{X: 2.3, Y: 1}, Vector3{Z: 1}, 0.5, 20)
	if !ok || hit.Body != wall || hit.Position.Z < 4 || hit.Position.Z > 4.6 {
		t.Errorf("a ball passing 0.3 m beside the wall's side touches its edge at %+v, %v", hit, ok)
	}
	if _, ok := w.SphereCast(Vector3{X: 2.6, Y: 1}, Vector3{Z: 1}, 0.5, 20); ok {
		t.Error("a ball passing 0.6 m beside the wall touched it")
	}
	if hit, ok := w.SphereCast(Vector3{Y: 0.2}, Vector3{X: 1}, 0.5, 3); !ok || hit.Distance != 0 {
		t.Errorf("a ball already in the floor hits it at %+v, want at once", hit)
	}

	// A body removed or moved is where the world says.
	w.Remove(wall)
	if _, ok := w.SphereCast(Vector3{Y: 1}, Vector3{Z: 1}, 0.5, 20); ok {
		t.Error("the removed wall is still hit")
	}
	lift := w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{X: 5, Y: 0.5}})
	w.SetTransform(lift, Transform{Position: Vector3{X: 5, Y: 3}})
	if hit, ok := w.Raycast(Vector3{X: 5, Y: 10}, Vector3{Y: -1}, 20); !ok || hit.Body != lift || !within(hit.Position.Y, 3.5, 1e-4) {
		t.Errorf("the lift, moved up 2.5 m, is hit at %+v", hit)
	}
}

func TestCharacterStandsWalksAndSteps(t *testing.T) {
	w, _ := floorWorld()
	c := &CharacterController{Position: Vector3{Y: 2}}
	var velocity Vector3
	for range 120 {
		velocity.Y -= 20.0 / 60
		c.Move(w, velocity.Scale(1.0/60))
		if c.OnGround {
			velocity.Y = 0
		}
	}
	if !c.OnGround || !within(c.Position.Y, 0, 0.01) || !near3(c.GroundNormal, Vector3{Y: 1}) {
		t.Fatalf("after falling, the character stands at %v, on ground %v, want on the floor", c.Position, c.OnGround)
	}

	// A wall stops it, and it slides along the wall.
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{Z: 3, Y: 1}, Scale: Vector3{X: 10, Y: 2, Z: 1}})
	for range 120 {
		c.Move(w, Vector3{X: 1, Z: 3}.Scale(1.0/60))
	}
	if !within(c.Position.Z, 2.5-0.3, 0.02) || c.Position.X < 1.5 || !c.OnWall || !c.OnGround {
		t.Errorf("walking into the wall at z = 2.5, the character is at %v (wall %v, ground %v), want stopped 0.3 m from it, slid along", c.Position, c.OnWall, c.OnGround)
	}

	// A low step it walks up; a high one stops it.
	c.Position = Vector3{X: -5, Z: -5}
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{X: -5, Y: 0.1, Z: -3}, Scale: Vector3{X: 2, Y: 0.2, Z: 2}})
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{X: 5, Y: 0.4, Z: -3}, Scale: Vector3{X: 2, Y: 0.8, Z: 2}})
	for range 90 {
		c.Move(w, Vector3{Z: 2.0 / 60, Y: -0.05})
	}
	if !within(c.Position.Y, 0.2, 0.02) || !c.OnGround {
		t.Errorf("walking onto a 0.2 m step, the character is at %v, on ground %v, want up on it", c.Position, c.OnGround)
	}
	c.Position = Vector3{X: 5, Z: -5}
	for range 90 {
		c.Move(w, Vector3{Z: 2.0 / 60, Y: -0.05})
	}
	if c.Position.Y > 0.01 || c.Position.Z > -4.3+0.01 {
		t.Errorf("walking into a 0.8 m block, the character is at %v, want stopped before it, on the floor", c.Position)
	}
}

func TestCharacterOnSlopesAndUnderCeilings(t *testing.T) {
	// A ramp of 30 degrees it walks up, and one of 60 it can't.
	for _, c := range []struct {
		degrees float32
		climbs  bool
	}{{30, true}, {60, false}} {
		w, _ := floorWorld()
		w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{Z: 3}, Rotation: Rotation{Pitch: c.degrees}, Scale: Vector3{X: 4, Y: 0.1, Z: 6}})
		body := &CharacterController{Position: Vector3{Z: -1}, OnGround: true}
		for range 180 {
			body.Move(w, Vector3{Z: 2.0 / 60, Y: -0.1})
		}
		if climbed := body.Position.Y > 1; climbed != c.climbs {
			t.Errorf("a %v° ramp: the character reached %v, climbing it is %v, want %v", c.degrees, body.Position, climbed, c.climbs)
		}
		if c.climbs {
			slope := float32(math.Cos(float64(c.degrees) * math.Pi / 180))
			if !body.OnGround || !within(body.GroundNormal.Y, slope, 0.01) {
				t.Errorf("on the %v° ramp: ground %v, normal %v", c.degrees, body.OnGround, body.GroundNormal)
			}
			// Down again, it keeps to the ramp rather than leaving it.
			for range 30 {
				body.Move(w, Vector3{Z: -3.0 / 60})
				if !body.OnGround {
					t.Fatalf("walking down the %v° ramp the character left the ground at %v", c.degrees, body.Position)
				}
			}
		}
	}

	w, _ := floorWorld()
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{Y: 2.5}, Scale: Vector3{X: 4, Y: 1, Z: 4}})
	body := &CharacterController{Height: 1.8}
	body.Move(w, Vector3{Y: 0.5})
	if !body.OnCeiling || !within(body.Position.Y, 0.2, 0.01) {
		t.Errorf("jumping under a ceiling at 2 m, the character is at %v, ceiling %v, want its head stopped against it", body.Position, body.OnCeiling)
	}
}

func TestPhysicsFromAModel(t *testing.T) {
	// A floor, and a pond marked collide = false in Blender.
	b := gltf.NewBuilder()
	square := b.Add("meshes", map[string]any{"primitives": []any{map[string]any{
		"attributes": map[string]any{
			"POSITION": b.Floats([]float32{-1, 0, 1, 1, 0, 1, 1, 0, -1, -1, 0, -1}, 3),
			"NORMAL":   b.Floats([]float32{0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0}, 3),
		},
		"indices": b.Indices([]uint16{0, 1, 2, 0, 2, 3}),
	}}})
	floor := b.Add("nodes", map[string]any{"name": "Floor", "mesh": square})
	pond := b.Add("nodes", map[string]any{"name": "Pond", "mesh": square, "translation": []float32{0, 1, 0},
		"extras": map[string]any{"collide": false}})
	b.Root(b.Add("nodes", map[string]any{"name": "Level", "children": []any{floor, pond}}))
	useAssets(t, map[string][]byte{"models/level.glb": b.GLB()})

	w := NewPhysicsWorld()
	level := w.AddModel(NewModel("models/level.glb"), Transform{Position: Vector3{Y: 2}})
	hit, ok := w.Raycast(Vector3{Y: 10}, Vector3{Y: -1}, 20)
	if !ok || hit.Body != level || !within(hit.Position.Y, 2, 1e-4) {
		t.Errorf("a ray down onto the level hits %+v, %v, want its floor at 2, through the pond", hit, ok)
	}
}

func TestCameraAvoidsWalls(t *testing.T) {
	w, _ := floorWorld()
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{Y: 1, Z: -3}, Scale: Vector3{X: 10, Y: 4, Z: 0.5}})
	camera := NewCamera3D(800, 600)
	camera.Orbit(Vector3{Y: 1}, 0, -10, 6) // behind the target, past the wall at z = -3, 3 m high
	camera.AvoidWalls(w)
	if _, hidden := w.Raycast(camera.Target, camera.Position.Sub(camera.Target), camera.Position.Distance(camera.Target)); hidden || camera.Position.Distance(camera.Target) < 2.9 {
		t.Errorf("the camera behind a 3 m wall is at %v, want risen over it, at least half its 6 m away, with nothing between it and its target", camera.Position)
	}
	// Under a ceiling it can't rise: it comes in front of the wall.
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{Y: 3.5}, Scale: Vector3{X: 20, Y: 1, Z: 20}})
	camera.Orbit(Vector3{Y: 1}, 0, -10, 6)
	camera.AvoidWalls(w)
	if camera.Position.Z < -2.75 || camera.Position.Z > -2 {
		t.Errorf("the camera behind a wall under a ceiling is at %v, want in front of the wall's face at z = -2.75", camera.Position)
	}
	camera.Orbit(Vector3{Y: 1}, 180, -10, 2)
	before := camera.Position
	camera.AvoidWalls(w)
	if camera.Position != before {
		t.Errorf("a camera with nothing in the way moved from %v to %v", before, camera.Position)
	}
}

func TestSoundsAreHeardFromTheCamera(t *testing.T) {
	camera := NewCamera3D(800, 600)
	camera.Orbit(Vector3{}, 0, 0, 5) // looking towards +Z: the screen's right is -X
	for _, c := range []struct {
		at          Vector3
		volume, pan float32
	}{
		{Vector3{Z: 1}, 1, 0},
		{Vector3{X: -10}, 0.2, 0.8},
		{Vector3{X: 10}, 0.2, -0.8},
		{Vector3{Z: 60}, 0, 0},
	} {
		volume, pan := heardFrom(c.at, camera)
		if !within(volume, c.volume, 0.01) || !within(pan, c.pan, 0.01) {
			t.Errorf("a sound at %v is heard at volume %v from %v, want %v from %v", c.at, volume, pan, c.volume, c.pan)
		}
	}
}

func TestNavMeshFindsTheWayRound(t *testing.T) {
	// A floor with a wall across it, a gap at its east end, and a pit to the
	// west; a raised platform reached by a ramp.
	w, _ := floorWorld()
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{X: -2, Y: 1}, Scale: Vector3{X: 14, Y: 2, Z: 0.5}}) // from x = -9 to 5
	nav := NewNavMesh(w, CharacterController{Height: 1.8, Radius: 0.4})
	from, to := Vector3{Z: -4}, Vector3{Z: 4}
	path, ok := nav.Path(from, to)
	if !ok {
		t.Fatal("found no way round the wall")
	}
	if path[0] != from || path[len(path)-1] != to {
		t.Errorf("the path runs from %v to %v, want from %v to %v", path[0], path[len(path)-1], from, to)
	}
	east := false
	for _, p := range path {
		east = east || p.X > 5
	}
	if !east || len(path) > 6 {
		t.Errorf("the path %v should go round the wall's east end, past x = 5, in a few straight lines", path)
	}
	// Each leg is walkable: nothing solid in the way.
	for i := 1; i < len(path); i++ {
		a, b := path[i-1].Add(Vector3{Y: 1}), path[i].Add(Vector3{Y: 1})
		if _, hit := w.Raycast(a, b.Sub(a), b.Sub(a).Length()); hit {
			t.Errorf("the leg from %v to %v goes through the wall", path[i-1], path[i])
		}
	}

	// Closed off, there is no way.
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{X: 7.5, Y: 1}, Scale: Vector3{X: 6, Y: 2, Z: 0.5}})
	w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{X: -9.5, Y: 1}, Scale: Vector3{X: 2, Y: 2, Z: 0.5}})
	if path, ok := NewNavMesh(w, CharacterController{Radius: 0.4}).Path(from, to); ok {
		t.Errorf("found a way %v through a wall from side to side", path)
	}
}

func TestCharacterStepsNoHigherThanStepHeight(t *testing.T) {
	for _, c := range []struct {
		ledge  float32
		climbs bool
	}{{0.25, true}, {0.35, false}, {0.5, false}} {
		w, _ := floorWorld()
		w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{Y: c.ledge / 2, Z: 6}, Scale: Vector3{X: 4, Y: c.ledge, Z: 10}})
		body := &CharacterController{Radius: 0.4, StepHeight: 0.3, OnGround: true}
		for range 120 {
			body.Move(w, Vector3{Z: 2.0 / 60, Y: -0.05})
		}
		if climbed := body.Position.Y > c.ledge-0.01; climbed != c.climbs {
			t.Errorf("a %v m ledge, step height 0.3, radius 0.4: the character ended at %v, climbing it is %v, want %v", c.ledge, body.Position, climbed, c.climbs)
		}
	}
}

func TestCharacterRidesALift(t *testing.T) {
	w, _ := floorWorld()
	lift := w.AddMesh(NewBoxMesh(), Transform{Position: Vector3{X: 3, Y: 0.25}, Scale: Vector3{X: 2, Y: 0.5, Z: 2}})
	body := &CharacterController{Position: Vector3{X: 3, Y: 0.5}}
	body.Move(w, Vector3{Y: -0.05})
	if !body.OnGround || body.GroundBody != lift {
		t.Fatalf("on the lift, ground %v on body %d, want body %d", body.OnGround, body.GroundBody, lift)
	}
	for frame := 1; frame <= 60; frame++ {
		w.SetTransform(lift, Transform{Position: Vector3{X: 3, Y: 0.25 + float32(frame)*0.05}, Scale: Vector3{X: 2, Y: 0.5, Z: 2}})
		body.Move(w, Vector3{Y: -0.05})
	}
	if !body.OnGround || !within(body.Position.Y, 3.5, 0.02) {
		t.Errorf("after the lift rose 3 m the character is at %v, on ground %v, want carried up on it, at 3.5", body.Position, body.OnGround)
	}
	if !within(body.Moved.Y, 0.05, 0.01) {
		t.Errorf("Moved = %v, want the lift's 5 cm a frame", body.Moved)
	}
}

func TestNavMeshPathToAPointInTheAir(t *testing.T) {
	w, _ := floorWorld()
	nav := NewNavMesh(w, CharacterController{Radius: 0.4})
	path, ok := nav.Path(Vector3{X: -3}, Vector3{X: 3, Y: 4})
	if !ok || !within(path[len(path)-1].Y, 0, 0.01) || !within(path[len(path)-1].X, 3, 0.3) {
		t.Errorf("a path to a point 4 m up ends at %v, %v, want on the floor under it", path, ok)
	}
}
