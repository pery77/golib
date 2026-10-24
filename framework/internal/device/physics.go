package device

// The rigid bodies of the contract: a world of bodies that fall, roll and
// bounce, simulated by Jolt Physics. On the desktop Jolt is in GoLib's
// native library, golibnative, called through purego; in a browser it is
// JoltPhysics.js, the same engine built for WebAssembly, which web builds
// carry when a game uses it. Package golib describes bodies and reads them
// back as arrays of floats, so one call carries a body or every body.

// Physics is a world of rigid bodies. Portable field: ID, 0 for none.
type Physics struct {
	ID uintptr
}

// PhysicsMarker is in the build of every game that makes a world of rigid
// bodies, and in no other: NewPhysics keeps it in usesPhysics. golib looks
// for it to copy GoLib's native library, or JoltPhysics.js, next to a game
// only when the game needs it. tools/cli has the same text.
const PhysicsMarker = "golib-uses-rigid-bodies:jolt-physics"

// usesPhysics holds PhysicsMarker once a world is made, which keeps the
// marker in the builds of the games that make one.
var usesPhysics string

// NoBody is the id of a body that couldn't be made.
const NoBody = 0xffffffff

// The floats of a body's description, for AddPhysicsBody, in order. A
// convex hull's points follow the description, three floats each.
const (
	BodyShape          = iota // ShapeBox, ShapeSphere, ShapeCapsule, ShapeHull or ShapeCylinder
	BodySizeX                 // a box's half size each way; a sphere's radius in X;
	BodySizeY                 // a capsule's radius in X and its middle's half
	BodySizeZ                 // height in Y
	BodyPositionX             // where it starts
	BodyPositionY             //
	BodyPositionZ             //
	BodyRotationX             // how it is turned, a quaternion x, y, z, w
	BodyRotationY             //
	BodyRotationZ             //
	BodyRotationW             //
	BodyMotion                // MotionStill, MotionMoved or MotionFree
	BodyMass                  // kilograms; 0 for its shape full of water
	BodyFriction              // 0 ice to 1 rubber
	BodyRestitution           // bounce: 0 none to 1 all of it
	BodyLinearDamping         // how the air slows it
	BodyAngularDamping        // and its spin
	BodyGravity               // how much gravity pulls it, 1 as everything
	BodyFast                  // 1 to check its whole way each step, for what is fast and small
	BodyPoints                // how many points a hull has
	BodyFloats                // how many floats a description takes, before the points
)

// The shapes a body can have.
const (
	ShapeBox = iota
	ShapeSphere
	ShapeCapsule
	ShapeHull
	ShapeCylinder // its radius in X and half its height in Y, as a capsule's
)

// How a body moves: never, as the game moves it, or as the physics does.
const (
	MotionStill = iota
	MotionMoved
	MotionFree
)

// BodyStateFloats is how many floats ReadPhysicsBodies writes for each body:
// its position, its rotation as a quaternion, its velocity, its spin, and 1
// when it is awake, 0 when it has come to rest.
const BodyStateFloats = 14

// What SetPhysicsBody changes, and the floats it takes for each.
const (
	SetPlace    = iota // position and rotation: 7
	SetVelocity        // velocity and spin: 6
	SetImpulse         // an impulse, and the point of the world it acts at: 6
	SetMoveTo          // for a body the game moves, where it is to be in dt: 7, then dt
	SetForce           // a force at its middle, for the next step: 3
)
