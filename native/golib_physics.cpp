// The rigid bodies of GoLib's native library: Jolt Physics behind a C
// interface small enough to call through purego, whose calls take whole
// numbers and pointers only. Every number goes through memory, as arrays of
// floats, so one call carries a body's whole description or every body's
// state. framework/internal/device/raylib_physics.go is the other side, and
// web.js does the same with JoltPhysics.js in a browser.

#include <Jolt/Jolt.h>

#include <Jolt/Core/Factory.h>
#include <Jolt/Core/JobSystemThreadPool.h>
#include <Jolt/Core/TempAllocator.h>
#include <Jolt/Physics/Body/BodyCreationSettings.h>
#include <Jolt/Physics/Collision/Shape/BoxShape.h>
#include <Jolt/Physics/Collision/Shape/CapsuleShape.h>
#include <Jolt/Physics/Collision/Shape/ConvexHullShape.h>
#include <Jolt/Physics/Collision/Shape/MeshShape.h>
#include <Jolt/Physics/Collision/Shape/RotatedTranslatedShape.h>
#include <Jolt/Physics/Collision/Shape/SphereShape.h>
#include <Jolt/Physics/PhysicsSettings.h>
#include <Jolt/Physics/PhysicsSystem.h>
#include <Jolt/RegisterTypes.h>

#include <algorithm>
#include <cstdarg>
#include <cstdint>
#include <cstdio>
#include <thread>

#if defined(_WIN32)
#define GOLIB_EXPORT extern "C" __declspec(dllexport)
#else
#define GOLIB_EXPORT extern "C" __attribute__((visibility("default")))
#endif

using namespace JPH;

namespace {

// Two layers: what never moves, and what does. Still things don't collide
// with each other.
namespace Layers {
constexpr ObjectLayer still = 0;
constexpr ObjectLayer moving = 1;
constexpr ObjectLayer count = 2;
} // namespace Layers

namespace BroadLayers {
constexpr BroadPhaseLayer still(0);
constexpr BroadPhaseLayer moving(1);
constexpr uint count = 2;
} // namespace BroadLayers

class BroadPhaseLayers final : public BroadPhaseLayerInterface {
public:
	uint GetNumBroadPhaseLayers() const override { return BroadLayers::count; }
	BroadPhaseLayer GetBroadPhaseLayer(ObjectLayer layer) const override {
		return layer == Layers::still ? BroadLayers::still : BroadLayers::moving;
	}
#if defined(JPH_EXTERNAL_PROFILE) || defined(JPH_PROFILE_ENABLED)
	const char *GetBroadPhaseLayerName(BroadPhaseLayer layer) const override {
		return layer == BroadLayers::still ? "still" : "moving";
	}
#endif
};

class ObjectVsBroadPhase final : public ObjectVsBroadPhaseLayerFilter {
public:
	bool ShouldCollide(ObjectLayer layer, BroadPhaseLayer broad) const override {
		return layer == Layers::moving || broad == BroadLayers::moving;
	}
};

class ObjectPairs final : public ObjectLayerPairFilter {
public:
	bool ShouldCollide(ObjectLayer a, ObjectLayer b) const override {
		return a == Layers::moving || b == Layers::moving;
	}
};

// world is one PhysicsSystem with what it needs to run.
struct world {
	BroadPhaseLayers broadLayers;
	ObjectVsBroadPhase objectVsBroad;
	ObjectPairs objectPairs;
	PhysicsSystem system;
	TempAllocatorImpl temp{16 * 1024 * 1024};
	JobSystemThreadPool jobs{cMaxPhysicsJobs, cMaxPhysicsBarriers,
		(int)std::max(1u, std::min(4u, std::thread::hardware_concurrency() > 1 ? std::thread::hardware_concurrency() - 1 : 1u))};
};

bool started = false;

// trace prints what Jolt says, on the game's console.
void trace(const char *format, ...) {
	va_list list;
	va_start(list, format);
	std::fprintf(stderr, "golibnative: ");
	std::vfprintf(stderr, format, list);
	std::fprintf(stderr, "\n");
	va_end(list);
}

#ifdef JPH_ENABLE_ASSERTS
bool assertFailed(const char *expression, const char *message, const char *file, uint line) {
	std::fprintf(stderr, "golibnative: Jolt: %s:%u: %s %s\n", file, line, expression, message ? message : "");
	return true;
}
#endif

// start registers Jolt's types, once.
void start() {
	if (started) {
		return;
	}
	started = true;
	Trace = trace;
	JPH_IF_ENABLE_ASSERTS(AssertFailed = assertFailed;)
	RegisterDefaultAllocator();
	Factory::sInstance = new Factory();
	RegisterTypes();
}

BodyID idOf(uint32_t id) { return BodyID(id); }

} // namespace

// golib_world_new makes a world for up to maxBodies bodies.
GOLIB_EXPORT void *golib_world_new(int32_t maxBodies) {
	start();
	world *w = new world();
	// Pairs and contacts far more than a game's busiest moment needs, and
	// few enough that each step's working memory fits the temp allocator.
	uint bodies = (uint)std::max(1024, maxBodies);
	w->system.Init(bodies, 0, std::min(bodies, 16384u), std::min(bodies, 8192u), w->broadLayers, w->objectVsBroad, w->objectPairs);
	w->system.SetGravity(Vec3(0, -9.81f, 0));
	return w;
}

// golib_world_free frees a world and everything in it.
GOLIB_EXPORT void golib_world_free(void *w) { delete (world *)w; }

// golib_world_gravity sets the pull on everything, from three floats.
GOLIB_EXPORT void golib_world_gravity(void *w, const float *g) {
	((world *)w)->system.SetGravity(Vec3(g[0], g[1], g[2]));
}

// golib_add_mesh adds a body made of triangles: count corners, three floats
// each, three corners a triangle, in the body's own space; then in more,
// its friction and restitution, its position and its rotation as a
// quaternion, and its motion: 0 still, 1 moved by the game (a lift). It
// returns the body's id, or 0xffffffff when the triangles make no shape.
GOLIB_EXPORT uint32_t golib_add_mesh(void *w, const float *corners, int32_t count, const float *more) {
	TriangleList triangles;
	triangles.reserve(count / 3);
	for (int32_t i = 0; i + 2 < count; i += 3) {
		const float *a = corners + i * 3;
		triangles.push_back(Triangle(Float3(a[0], a[1], a[2]), Float3(a[3], a[4], a[5]), Float3(a[6], a[7], a[8])));
	}
	MeshShapeSettings settings(triangles);
	Shape::ShapeResult shape = settings.Create();
	if (shape.HasError()) {
		return BodyID::cInvalidBodyID;
	}
	bool moved = more[9] == 1;
	BodyCreationSettings body(shape.Get(), RVec3(more[2], more[3], more[4]), Quat(more[5], more[6], more[7], more[8]).Normalized(),
		moved ? EMotionType::Kinematic : EMotionType::Static, moved ? Layers::moving : Layers::still);
	body.mFriction = more[0];
	body.mRestitution = more[1];
	BodyInterface &bodies = ((world *)w)->system.GetBodyInterface();
	return bodies.CreateAndAddBody(body, moved ? EActivation::Activate : EActivation::DontActivate).GetIndexAndSequenceNumber();
}

// The description golib_add_body reads, as floats.
enum : int {
	dShape,      // 0 box, 1 sphere, 2 capsule, 3 convex hull of points
	dSizeX,      // a box's half extents; a sphere's radius in X; a capsule's
	dSizeY,      // radius in X and the half height of its middle in Y
	dSizeZ,
	dPositionX,
	dPositionY,
	dPositionZ,
	dRotationX, // a quaternion
	dRotationY,
	dRotationZ,
	dRotationW,
	dMotion, // 0 still, 1 moved by the game, 2 moved by the physics
	dMass,   // kilograms; 0 works it out from the shape at 1000 kg/m³
	dFriction,
	dRestitution,
	dLinearDamping,
	dAngularDamping,
	dGravity, // how much gravity pulls it: 1 as everything else
	dFast,    // 1 for continuous collision, for what moves fast and small
	dPoints,  // a convex hull's number of points, which follow
	dCount
};

// golib_add_body adds a body of the shape a description says, and returns
// its id, or 0xffffffff when the shape is impossible.
GOLIB_EXPORT uint32_t golib_add_body(void *w, const float *d) {
	Shape::ShapeResult shape;
	switch ((int)d[dShape]) {
	case 0:
		shape = BoxShapeSettings(Vec3(d[dSizeX], d[dSizeY], d[dSizeZ]), std::min(0.05f, 0.5f * std::min({d[dSizeX], d[dSizeY], d[dSizeZ]}))).Create();
		break;
	case 1:
		shape = SphereShapeSettings(d[dSizeX]).Create();
		break;
	case 2:
		shape = CapsuleShapeSettings(d[dSizeY], d[dSizeX]).Create();
		break;
	case 3: {
		Array<Vec3> points;
		int count = (int)d[dPoints];
		for (int i = 0; i < count; i++) {
			const float *p = d + dCount + i * 3;
			points.push_back(Vec3(p[0], p[1], p[2]));
		}
		shape = ConvexHullShapeSettings(points, 0.02f).Create();
		break;
	}
	}
	if (shape.HasError()) {
		return BodyID::cInvalidBodyID;
	}
	EMotionType motion = d[dMotion] == 2 ? EMotionType::Dynamic : (d[dMotion] == 1 ? EMotionType::Kinematic : EMotionType::Static);
	BodyCreationSettings body(shape.Get(), RVec3(d[dPositionX], d[dPositionY], d[dPositionZ]),
		Quat(d[dRotationX], d[dRotationY], d[dRotationZ], d[dRotationW]).Normalized(),
		motion, motion == EMotionType::Static ? Layers::still : Layers::moving);
	if (d[dMass] > 0) {
		body.mOverrideMassProperties = EOverrideMassProperties::CalculateInertia;
		body.mMassPropertiesOverride.mMass = d[dMass];
	}
	body.mFriction = d[dFriction];
	body.mRestitution = d[dRestitution];
	body.mLinearDamping = d[dLinearDamping];
	body.mAngularDamping = d[dAngularDamping];
	body.mGravityFactor = d[dGravity];
	if (d[dFast] != 0) {
		body.mMotionQuality = EMotionQuality::LinearCast;
	}
	BodyInterface &bodies = ((world *)w)->system.GetBodyInterface();
	EActivation activation = motion == EMotionType::Static ? EActivation::DontActivate : EActivation::Activate;
	return bodies.CreateAndAddBody(body, activation).GetIndexAndSequenceNumber();
}

// golib_remove_body takes a body out and frees it.
GOLIB_EXPORT void golib_remove_body(void *w, uint32_t id) {
	BodyInterface &bodies = ((world *)w)->system.GetBodyInterface();
	bodies.RemoveBody(idOf(id));
	bodies.DestroyBody(idOf(id));
}

// golib_step moves the world on by time[0] seconds, in time[1] collision
// steps.
GOLIB_EXPORT void golib_step(void *w, const float *time) {
	world *x = (world *)w;
	x->system.Update(time[0], std::max(1, (int)time[1]), &x->temp, &x->jobs);
}

// golib_read writes, for each of count bodies, 14 floats into out: its
// position, its rotation as a quaternion, its velocity, its spin, and 1 when
// it is awake.
GOLIB_EXPORT void golib_read(void *w, const uint32_t *ids, int32_t count, float *out) {
	const BodyInterface &bodies = ((world *)w)->system.GetBodyInterfaceNoLock();
	for (int32_t i = 0; i < count; i++) {
		BodyID id = idOf(ids[i]);
		float *o = out + i * 14;
		RVec3 p;
		Quat q;
		bodies.GetPositionAndRotation(id, p, q);
		Vec3 v = bodies.GetLinearVelocity(id);
		Vec3 s = bodies.GetAngularVelocity(id);
		o[0] = (float)p.GetX(), o[1] = (float)p.GetY(), o[2] = (float)p.GetZ();
		o[3] = q.GetX(), o[4] = q.GetY(), o[5] = q.GetZ(), o[6] = q.GetW();
		o[7] = v.GetX(), o[8] = v.GetY(), o[9] = v.GetZ();
		o[10] = s.GetX(), o[11] = s.GetY(), o[12] = s.GetZ();
		o[13] = bodies.IsActive(id) ? 1.0f : 0.0f;
	}
}

// What golib_set changes, from the floats it is given.
enum : int {
	setPlace,    // position and rotation: 7 floats
	setVelocity, // velocity and spin: 6 floats
	setImpulse,  // an impulse at a point of the world: 6 floats
	setMoveTo,   // for a body the game moves: where to be after dt, 7 floats then dt
	setForce,    // a force for the next step at its middle: 3 floats
};

// golib_set changes a body as what says.
GOLIB_EXPORT void golib_set(void *w, uint32_t id, int32_t what, const float *v) {
	BodyInterface &bodies = ((world *)w)->system.GetBodyInterface();
	BodyID body = idOf(id);
	switch (what) {
	case setPlace:
		bodies.SetPositionAndRotation(body, RVec3(v[0], v[1], v[2]), Quat(v[3], v[4], v[5], v[6]).Normalized(), EActivation::Activate);
		break;
	case setVelocity:
		bodies.SetLinearAndAngularVelocity(body, Vec3(v[0], v[1], v[2]), Vec3(v[3], v[4], v[5]));
		break;
	case setImpulse:
		bodies.AddImpulse(body, Vec3(v[0], v[1], v[2]), RVec3(v[3], v[4], v[5]));
		break;
	case setMoveTo:
		bodies.MoveKinematic(body, RVec3(v[0], v[1], v[2]), Quat(v[3], v[4], v[5], v[6]).Normalized(), v[7]);
		break;
	case setForce:
		bodies.AddForce(body, Vec3(v[0], v[1], v[2]));
		break;
	}
}

// golib_version returns the version of this library's interface, which the
// Go side checks: 1 is this one.
GOLIB_EXPORT int32_t golib_version() { return 1; }
