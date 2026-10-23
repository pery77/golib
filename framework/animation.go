package golib

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// A model's animations move its nodes: the bones of a skeleton, which bend
// the meshes skinned to them, and any other node, such as a door or a
// windmill's sails. They play through an Animator, one for each thing that
// moves, which blends them, layers them and poses the model; the model
// itself stays as the file made it, shared by everything drawn with it.

// quaternion is a turn as glTF keeps it, x, y, z, w: what animations store
// and blend, since three angles can't be blended smoothly. Games see
// Rotation.
type quaternion [4]float32

var noTurn = quaternion{0, 0, 0, 1}

func (q quaternion) dot(r quaternion) float32 {
	return q[0]*r[0] + q[1]*r[1] + q[2]*r[2] + q[3]*r[3]
}

func (q quaternion) normalize() quaternion {
	length := float32(math.Sqrt(float64(q.dot(q))))
	if length == 0 {
		return noTurn
	}
	return quaternion{q[0] / length, q[1] / length, q[2] / length, q[3] / length}
}

// slerp turns from a to b by t, the short way round, at an even speed.
func slerp(a, b quaternion, t float32) quaternion {
	cos := a.dot(b)
	if cos < 0 {
		b, cos = quaternion{-b[0], -b[1], -b[2], -b[3]}, -cos
	}
	wa, wb := 1-t, t
	if cos < 0.9995 {
		angle := math.Acos(float64(cos))
		sin := math.Sin(angle)
		wa = float32(math.Sin(float64(1-t)*angle) / sin)
		wb = float32(math.Sin(float64(t)*angle) / sin)
	}
	return quaternion{wa*a[0] + wb*b[0], wa*a[1] + wb*b[1], wa*a[2] + wb*b[2], wa*a[3] + wb*b[3]}.normalize()
}

// nodePose is where a node is relative to its parent: moved, turned and
// scaled, in that order of reading, scale applied first.
type nodePose struct {
	translation Vector3
	rotation    quaternion
	scale       Vector3
}

// matrix returns the pose as a matrix: scaled, then turned, then moved.
func (p nodePose) matrix() matrix {
	x, y, z, w := p.rotation[0], p.rotation[1], p.rotation[2], p.rotation[3]
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z
	s := p.scale
	return matrix{
		(1 - 2*(yy+zz)) * s.X, 2 * (xy + wz) * s.X, 2 * (xz - wy) * s.X, 0,
		2 * (xy - wz) * s.Y, (1 - 2*(xx+zz)) * s.Y, 2 * (yz + wx) * s.Y, 0,
		2 * (xz + wy) * s.Z, 2 * (yz - wx) * s.Z, (1 - 2*(xx+yy)) * s.Z, 0,
		p.translation.X, p.translation.Y, p.translation.Z, 1,
	}
}

// modelNode is a node of a model's tree, as the file has it.
type modelNode struct {
	name     string
	parent   int // -1 at the top
	rest     nodePose
	local    matrix // where it is relative to its parent, at rest
	byMatrix bool   // the file placed it with a matrix, which animations never move
	mesh     bool   // it carries one of the model's meshes
	weights  []float32
}

// modelSkin binds meshes to bones.
type modelSkin struct {
	joints      []int    // the nodes that are its bones
	inverseBind []matrix // from the mesh's space into each bone's, as bound
	root        int      // its top bone, which root motion follows
	under       []int    // the root and every node below it
}

// channelPath is what a channel animates.
type channelPath int

const (
	pathTranslation channelPath = iota
	pathRotation
	pathScale
	pathWeights
)

// animationChannel animates one property of one node.
type animationChannel struct {
	node   int
	path   channelPath
	step   bool // each value holds until the next
	cubic  bool // each keyframe is in-tangent, value, out-tangent
	times  []float32
	values []float32
	width  int // numbers in one value
}

// modelAnimation is one of a model's animations, as Blender names its
// actions.
type modelAnimation struct {
	name     string
	length   float32 // seconds, its last keyframe
	channels []animationChannel
	root     []int // its channels that move the skin's root bone, for root motion
}

// sample writes the channel's value at time seconds into out, width numbers.
func (c *animationChannel) sample(time float32, out []float32) {
	n := len(c.times)
	per := c.width
	at := 0 // where a keyframe's value starts within it
	if c.cubic {
		per, at = c.width*3, c.width
	}
	value := func(k int) []float32 { return c.values[k*per+at : k*per+at+c.width] }
	if n == 1 || time <= c.times[0] {
		copy(out, value(0))
		return
	}
	if time >= c.times[n-1] {
		copy(out, value(n-1))
		return
	}
	k := sort.Search(n, func(i int) bool { return c.times[i] > time }) - 1
	t0, t1 := c.times[k], c.times[k+1]
	u := float32(0)
	if t1 > t0 {
		u = (time - t0) / (t1 - t0)
	}
	a, b := value(k), value(k+1)
	switch {
	case c.step:
		copy(out, a)
	case c.cubic:
		// Hermite's curve, its tangents scaled by the keyframes' distance.
		gap := t1 - t0
		leaving := c.values[k*per+2*c.width:]
		arriving := c.values[(k+1)*per:]
		u2, u3 := u*u, u*u*u
		h00, h10 := 2*u3-3*u2+1, u3-2*u2+u
		h01, h11 := -2*u3+3*u2, u3-u2
		for i := range c.width {
			out[i] = h00*a[i] + h10*gap*leaving[i] + h01*b[i] + h11*gap*arriving[i]
		}
		if c.path == pathRotation {
			q := quaternion(out[:4]).normalize()
			copy(out, q[:])
		}
	case c.path == pathRotation:
		q := slerp(quaternion(a), quaternion(b), u)
		copy(out, q[:])
	default:
		for i := range c.width {
			out[i] = a[i] + (b[i]-a[i])*u
		}
	}
}

// poseAt sets the nodes the animation moves to where it has them at time
// seconds; the rest keep what poses and weights already hold.
func (a *modelAnimation) poseAt(time float32, poses []nodePose, weights [][]float32) {
	var value [4]float32
	for i := range a.channels {
		c := &a.channels[i]
		switch c.path {
		case pathTranslation:
			c.sample(time, value[:3])
			poses[c.node].translation = Vector3{X: value[0], Y: value[1], Z: value[2]}
		case pathRotation:
			c.sample(time, value[:4])
			poses[c.node].rotation = quaternion(value)
		case pathScale:
			c.sample(time, value[:3])
			poses[c.node].scale = Vector3{X: value[0], Y: value[1], Z: value[2]}
		case pathWeights:
			if w := weights[c.node]; len(w) == c.width {
				c.sample(time, w)
			}
		}
	}
}

// AnimationOptions say how [Animator.PlayWith] plays an animation. The zero
// value is what [Animator.Play] does: fade into it in 0.2 seconds and loop
// it at its own speed, over the whole model.
type AnimationOptions struct {
	// Fade is how many seconds it takes to blend from what played before
	// into this animation: 0.2 when zero. The first animation an Animator
	// plays starts at once.
	Fade float32

	// Speed multiplies how fast it plays: 1 when zero, 2 twice as fast, -1
	// backwards. Play it again with another Speed to change it as it plays,
	// such as a walk that follows how fast the character moves.
	Speed float32

	// Once plays it to its end and holds its last pose there, instead of
	// looping: a jump, an attack, a fall. [Animator.Finished] says when it
	// is over.
	Once bool

	// Then, with Once, is the animation to fade into when this one ends,
	// such as "Idle" after "Attack".
	Then string

	// Bone plays it only on the bone of this name and the bones below it,
	// over what plays on the rest: "Spine" waves the arms while the legs
	// keep walking. Each Bone is a layer of its own; the empty Bone is the
	// whole model, and the layer played last goes over the others. Any
	// node of the model can be named, and the shape keys an animation moves
	// are moved over the rest whatever the Bone: a face's expression over
	// the body's walk. A layer played Once without Then fades out at its
	// end, back to what plays under it.
	Bone string

	// Restart plays it again from its start even when it is already
	// playing, such as a second punch.
	Restart bool
}

// defaultFade is how long Play blends one animation into the next.
const defaultFade = 0.2

// Animator plays a model's animations for one thing that moves: one for each
// character, enemy or door, since each is in its own pose. Make it once,
// tell it what to play, move it on in Update and draw the model through it:
//
//	hero := golib.NewModel("models/hero.glb")
//	p.animator = golib.NewAnimator(hero) // in the player, once
//
//	// In Update: say what plays, every update; the same name goes on playing.
//	if p.speed > 0.1 {
//		p.animator.Play("Run")
//	} else {
//		p.animator.Play("Idle")
//	}
//	p.animator.Update(dt)
//
//	// In Draw, between Screen.Begin3D and Screen.End3D:
//	screen.DrawAnimated(p.animator, golib.Transform{Position: p.position, Rotation: golib.Rotation{Yaw: p.yaw}}, golib.White)
//
// Animations play by the names Blender gives its actions; [Model.Animations]
// lists them. Playing another fades into it, so calling Play from the game's
// own state every update is all a game needs.
type Animator struct {
	// RootMotion takes the movement of the model's top bone across the
	// ground out of its pose and gives it to [Animator.Moved], for the game
	// to move the model by: feet that don't slide, in a walk animated
	// moving forward. Up and down stays in the pose.
	RootMotion bool

	model  *Model
	layers []animationLayer // the whole model's first, then one for each Bone
	shapes map[string]float32
	moved  Vector3

	posed bool // pose holds the animations where they are now
	pose  modelPose

	// What posing works in, kept from one frame to the next.
	base, clip, over                      []nodePose
	baseWeights, clipWeights, overWeights [][]float32
}

// animationLayer is the animations playing on part of a model: the whole of
// it, or one bone and those below it.
type animationLayer struct {
	bone   string
	nodes  []bool // the nodes it moves, nil for all
	weight float32
	fading float32 // weight gained each second; negative while fading out
	clips  []playingClip
}

// playingClip is one animation playing on a layer, the newest last.
type playingClip struct {
	animation int // -1 for the model at rest
	time      float32
	speed     float32
	once      bool
	then      string
	fade      float32 // seconds, to fade into then
	weight    float32
	fading    float32 // weight gained each second; negative while fading out
	ended     bool    // a clip played once has reached its end
	followed  bool    // and what comes after it has started
}

// modelPose is a model posed: each node where it is, the morph weights of
// its meshes, and the matrices that bend its skins.
type modelPose struct {
	locals  []nodePose
	weights [][]float32 // for each node with a mesh with morph targets
	globals []matrix    // each node in the model's space
	joints  [][]float32 // for each skin, 16 numbers for each joint: global times inverse bind
}

// NewAnimator returns an Animator for model, showing it at rest until it
// plays something.
func NewAnimator(model *Model) *Animator {
	if model == nil {
		reportError(errors.New("golib: NewAnimator got a nil model: make one with golib.NewModel"))
		model = &Model{err: errors.New("golib: NewAnimator got a nil model")}
	}
	return &Animator{model: model, layers: []animationLayer{{weight: 1}}}
}

// Animations returns the names of the model's animations, in the file's
// order: Blender's actions, as its glTF export names them.
func (m *Model) Animations() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return nil
	}
	names := make([]string, len(m.animations))
	for i, a := range m.animations {
		names[i] = a.name
	}
	return names
}

// AnimationLength returns how many seconds the animation called name lasts,
// played once at its own speed, or 0 when the model has none of that name.
func (m *Model) AnimationLength(name string) float32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return 0
	}
	if i := m.animationIndex(name); i >= 0 {
		return m.animations[i].length
	}
	return 0
}

// Bones returns the names of the model's bones, the joints of its skins, in
// the file's order, each once: what AnimationOptions.Bone and Animator.Bone
// take. Markers parented to bones in Blender aren't among them; Markers
// lists those.
func (m *Model) Bones() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.prepare() != nil {
		return nil
	}
	return m.boneNames()
}

// ready reads the model, if it hasn't been, and reports what stopped it.
func (a *Animator) ready() bool {
	a.model.mu.Lock()
	err := a.model.prepare()
	a.model.mu.Unlock()
	if err != nil {
		reportError(err)
		return false
	}
	return true
}

// Play fades into the animation called name, and loops it, over the whole
// model. Playing the animation already playing changes nothing, so a game
// can call Play with what should play every update.
func (a *Animator) Play(name string) {
	a.PlayWith(name, AnimationOptions{})
}

// PlayWith plays the animation called name as options say: how long it
// fades in, how fast it plays, once or looping, and on which bones.
//
//	p.animator.PlayWith("Jump", golib.AnimationOptions{Once: true, Then: "Fall"})
//	p.animator.PlayWith("Wave", golib.AnimationOptions{Bone: "Spine", Once: true})
//	p.animator.PlayWith("Walk", golib.AnimationOptions{Speed: p.speed / walkSpeed})
func (a *Animator) PlayWith(name string, options AnimationOptions) {
	if !a.ready() {
		return
	}
	m := a.model
	index := m.animationIndex(name)
	if index < 0 {
		reportError(fmt.Errorf("golib: Animator.Play(%q): %s has no animation called %q; it has %s", name, m.call(), name, m.animationList()))
		return
	}
	layer := a.layerFor(options.Bone)
	if layer == nil {
		return
	}
	fade := options.Fade
	if fade <= 0 {
		fade = defaultFade
	}
	speed := options.Speed
	if speed == 0 {
		speed = 1
	}
	if n := len(layer.clips); n > 0 && !options.Restart {
		if newest := &layer.clips[n-1]; newest.animation == index && newest.fading >= 0 {
			newest.speed, newest.once, newest.then, newest.fade = speed, options.Once, options.Then, fade
			if layer.fading < 0 {
				layer.fading = 1 / fade
			}
			return
		}
	}
	layer = a.raise(layer)
	a.start(layer, playingClip{animation: index, speed: speed, once: options.Once, then: options.Then, fade: fade}, fade)
}

// raise puts a bone's layer over the others, as the one played last, and
// returns where it is now.
func (a *Animator) raise(layer *animationLayer) *animationLayer {
	if layer.nodes == nil {
		return layer
	}
	for i := range a.layers {
		if &a.layers[i] == layer {
			moved := a.layers[i]
			a.layers = append(append(a.layers[:i:i], a.layers[i+1:]...), moved)
			break
		}
	}
	return &a.layers[len(a.layers)-1]
}

// start begins clip on layer, fading the clips already there out in fade
// seconds. The first clip of the whole model's layer shows at once.
func (a *Animator) start(layer *animationLayer, clip playingClip, fade float32) {
	if clip.speed < 0 && clip.animation >= 0 {
		clip.time = a.model.animations[clip.animation].length
	}
	if layer.nodes == nil && len(layer.clips) == 0 {
		clip.weight = 1
	} else {
		clip.fading = 1 / fade
		for i := range layer.clips {
			layer.clips[i].fading = -1 / fade
		}
	}
	layer.clips = append(layer.clips, clip)
	if layer.nodes != nil && (layer.weight < 1 || layer.fading < 0) {
		layer.fading = 1 / fade
	}
	a.posed = false
}

// Stop fades out what plays on the layer of bone, back to what plays under
// it, in 0.2 seconds; Stop("") fades the whole model back to its rest pose.
func (a *Animator) Stop(bone string) {
	for i := range a.layers {
		layer := &a.layers[i]
		if layer.bone != bone {
			continue
		}
		if layer.nodes == nil {
			if n := len(layer.clips); n > 0 && layer.clips[n-1].animation >= 0 {
				a.start(layer, playingClip{animation: -1, speed: 1}, defaultFade)
			}
		} else {
			layer.fading = -1 / defaultFade
		}
	}
}

// layerFor returns the layer of bone, starting it if there is none yet, or
// nil after reporting a bone the model doesn't have.
func (a *Animator) layerFor(bone string) *animationLayer {
	for i := range a.layers {
		if a.layers[i].bone == bone {
			return &a.layers[i]
		}
	}
	m := a.model
	top := m.nodeIndex(bone)
	if top < 0 {
		reportError(fmt.Errorf("golib: AnimationOptions.Bone %q: %s has no bone called %q; it has %s", bone, m.call(), bone, m.boneList()))
		return nil
	}
	nodes := make([]bool, len(m.nodes))
	for _, n := range m.order {
		nodes[n] = n == top || (m.nodes[n].parent >= 0 && nodes[m.nodes[n].parent])
	}
	a.layers = append(a.layers, animationLayer{bone: bone, nodes: nodes})
	return &a.layers[len(a.layers)-1]
}

// Update moves the animations on by dt seconds, as Scene.Update gets it:
// call it in every Update.
func (a *Animator) Update(dt float32) {
	if !a.ready() {
		return
	}
	a.moved = Vector3{}
	m := a.model
	root := m.rootBone()
	var moved Vector3
	var total float32
	for li := 0; li < len(a.layers); li++ {
		layer := &a.layers[li]
		for ci := range layer.clips {
			clip := &layer.clips[ci]
			delta := a.advance(clip, dt, li == 0 && a.RootMotion && root >= 0)
			if li == 0 {
				moved = moved.Add(delta.Scale(clip.weight))
				total += clip.weight
			}
			clip.weight = clampUnit(clip.weight + clip.fading*dt)
		}
		// A clip played once that has just ended goes on to Then, or, on a
		// bone's layer, fades the layer out.
		if n := len(layer.clips); n > 0 {
			newest := &layer.clips[n-1]
			if newest.ended && newest.fading >= 0 && !newest.followed {
				newest.followed = true
				switch {
				case newest.then != "":
					then := m.animationIndex(newest.then)
					if then < 0 {
						reportError(fmt.Errorf("golib: AnimationOptions.Then %q: %s has no animation called %q; it has %s", newest.then, m.call(), newest.then, m.animationList()))
						break
					}
					a.start(layer, playingClip{animation: then, speed: 1, fade: newest.fade}, newest.fade)
					layer = &a.layers[li]
				case layer.nodes != nil:
					layer.fading = -1 / newest.fade
				}
			}
		}
		kept := layer.clips[:0]
		for i, clip := range layer.clips {
			if clip.weight > 0 || clip.fading > 0 || i == len(layer.clips)-1 {
				kept = append(kept, clip)
			}
		}
		layer.clips = kept
		if layer.nodes != nil {
			layer.weight = clampUnit(layer.weight + layer.fading*dt)
			if layer.weight == 0 && layer.fading < 0 {
				a.layers = append(a.layers[:li], a.layers[li+1:]...)
				li--
			}
		}
	}
	if total > 0 {
		a.moved = moved.Scale(1 / total)
	}
	a.posed = false
}

func clampUnit(v float32) float32 {
	return max(0, min(1, v))
}

// advance moves clip on by dt seconds, looping or holding at its end, and
// returns how far that moved the root bone across the ground, in the
// model's space, when measuring is true.
func (a *Animator) advance(clip *playingClip, dt float32, measuring bool) Vector3 {
	if clip.animation < 0 {
		return Vector3{}
	}
	anim := &a.model.animations[clip.animation]
	length := anim.length
	from := clip.time
	to := from + dt*clip.speed
	var wraps float32
	switch {
	case length <= 0:
		to = 0
	case clip.once:
		if to >= length || to <= 0 {
			clip.ended = to >= length && clip.speed > 0 || to <= 0 && clip.speed < 0
			to = max(0, min(length, to))
		}
	default:
		wraps = float32(math.Floor(float64(to / length)))
		to -= wraps * length
		if to >= length { // rounding
			to = 0
		}
	}
	clip.time = to
	if !measuring || len(anim.root) == 0 {
		return Vector3{}
	}
	at := func(t float32) Vector3 { return a.model.rootAt(anim, t) }
	delta := at(to).Sub(at(from))
	if wraps != 0 {
		delta = delta.Add(at(length).Sub(at(0)).Scale(wraps))
	}
	delta.Y = 0
	return delta
}

// Moved returns how far the animations carried the model's top bone across
// the ground in the last Update, with RootMotion on: in the model's own
// space, so turn it as the model is turned, and scale it as it is scaled.
//
//	p.animator.Update(dt)
//	p.position = p.position.Add(p.animator.Moved().Rotate(golib.Rotation{Yaw: p.yaw}))
func (a *Animator) Moved() Vector3 {
	return a.moved
}

// Playing returns the name of the animation the whole model plays, or fades
// into: the last one played without a Bone. It is "" at rest.
func (a *Animator) Playing() string {
	clips := a.layers[0].clips
	if len(clips) == 0 || clips[len(clips)-1].animation < 0 {
		return ""
	}
	return a.model.animations[clips[len(clips)-1].animation].name
}

// Time returns how many seconds into its animation Playing is: from 0 to its
// length, starting again as it loops.
func (a *Animator) Time() float32 {
	clips := a.layers[0].clips
	if len(clips) == 0 {
		return 0
	}
	return clips[len(clips)-1].time
}

// PlayingOn returns the name of the animation playing on the layer of bone,
// as Playing does for the whole model: "" once the layer has faded out, as
// one played Once without Then does at its end.
func (a *Animator) PlayingOn(bone string) string {
	for i := range a.layers {
		layer := &a.layers[i]
		if layer.bone != bone || layer.fading < 0 && layer.nodes != nil {
			continue
		}
		if clips := layer.clips; len(clips) > 0 && clips[len(clips)-1].animation >= 0 {
			return a.model.animations[clips[len(clips)-1].animation].name
		}
	}
	return ""
}

// TimeOn returns how many seconds into its animation the layer of bone is,
// as Time does for the whole model, or 0 when nothing plays there.
func (a *Animator) TimeOn(bone string) float32 {
	for i := range a.layers {
		if layer := &a.layers[i]; layer.bone == bone && len(layer.clips) > 0 {
			return layer.clips[len(layer.clips)-1].time
		}
	}
	return 0
}

// Finished says whether the animation Playing was played Once and has
// reached its end, holding its last pose: the attack is over.
func (a *Animator) Finished() bool {
	clips := a.layers[0].clips
	return len(clips) > 0 && clips[len(clips)-1].ended
}

// SetShapeKey sets the weight of the shape key called name, Blender's name
// for a morph target, from 0, as modeled, to 1, fully the shape: a smile, a
// blink, a bulge. It holds over what the animations give it, in every mesh
// of the model that has a shape key of that name.
func (a *Animator) SetShapeKey(name string, weight float32) {
	if !a.ready() {
		return
	}
	if !a.model.hasShapeKey(name) {
		reportError(fmt.Errorf("golib: Animator.SetShapeKey(%q): %s has no shape key called %q; it has %s", name, a.model.call(), name, a.model.shapeKeyList()))
		return
	}
	if a.shapes == nil {
		a.shapes = map[string]float32{}
	}
	a.shapes[name] = weight
	a.posed = false
}

// Bone returns where the bone called name is, in the model's own space, as
// the animations have it now: place what the model holds with
// [Transform.Child], such as a sword in a hand. A marker placed under a bone
// in Blender, an empty parented to it, follows it too, and can be asked for
// by its name: the best way to say where a held thing sits and how it is
// turned. The bone's scale is left out, so what is placed there keeps its
// own size.
//
//	hand := p.animator.Bone("Hand.R")
//	screen.DrawModel(sword, body.Child(hand), golib.White)
func (a *Animator) Bone(name string) Transform {
	if !a.ready() {
		return Transform{}
	}
	m := a.model
	n := m.nodeIndex(name)
	if n < 0 {
		reportError(fmt.Errorf("golib: Animator.Bone(%q): %s has no bone or marker called %q; its bones are %s", name, m.call(), name, m.boneList()))
		return Transform{}
	}
	position, rotation, _ := a.posedNow().globals[n].decompose()
	return Transform{Position: position, Rotation: rotation}
}

// posedNow returns the model posed as the animations have it now, working it
// out when they have moved since it last was.
func (a *Animator) posedNow() *modelPose {
	if a.posed {
		return &a.pose
	}
	m := a.model
	p := &a.pose
	if p.locals == nil {
		p.locals = make([]nodePose, len(m.nodes))
		p.weights = make([][]float32, len(m.nodes))
		for n, node := range m.nodes {
			if node.weights != nil {
				p.weights[n] = make([]float32, len(node.weights))
			}
		}
		p.globals = make([]matrix, len(m.nodes))
		p.joints = make([][]float32, len(m.skins))
		for s, skin := range m.skins {
			p.joints[s] = make([]float32, 16*len(skin.joints))
		}
	}
	m.restPose(p.locals, p.weights)
	for i := range a.layers {
		layer := &a.layers[i]
		if i == 0 {
			a.blend(layer, p.locals, p.weights)
			continue
		}
		if layer.weight == 0 {
			continue
		}
		over := growPoses(&a.over, len(m.nodes))
		copy(over, p.locals)
		overWeights := copyWeights(&a.overWeights, p.weights)
		a.blend(layer, over, overWeights)
		for n, moves := range layer.nodes {
			if moves {
				p.locals[n] = mixPose(p.locals[n], over[n], layer.weight)
			}
			// Shape keys belong to meshes, not bones: what the layer's
			// animations do to them goes over the rest too.
			if w := p.weights[n]; w != nil && (moves || a.shapesOn(layer, n)) {
				for k := range w {
					w[k] += (overWeights[n][k] - w[k]) * layer.weight
				}
			}
		}
	}
	for name, weight := range a.shapes {
		m.setShapeKey(p.weights, name, weight)
	}
	m.place(p)
	if a.RootMotion {
		if root := m.rootBone(); root >= 0 {
			rest := m.rest.globals[root]
			offset := Vector3{X: p.globals[root][12] - rest[12], Z: p.globals[root][14] - rest[14]}
			for _, n := range m.skins[m.rootSkin].under {
				p.globals[n][12] -= offset.X
				p.globals[n][14] -= offset.Z
			}
		}
	}
	m.bend(p)
	a.posed = true
	return p
}

// blend sets poses, and weights unless it is nil, to the layer's clips mixed
// by their weights. Nodes no clip moves stay as poses has them.
func (a *Animator) blend(layer *animationLayer, poses []nodePose, weights [][]float32) {
	m := a.model
	var total float32
	for _, clip := range layer.clips {
		total += clip.weight
	}
	if total <= 0 {
		return
	}
	base := growPoses(&a.base, len(poses))
	copy(base, poses)
	baseWeights := copyWeights(&a.baseWeights, weights)
	for _, w := range weights {
		clear(w)
	}
	clipPoses := growPoses(&a.clip, len(poses))
	clipWeights := copyWeights(&a.clipWeights, weights)
	var sum float32
	for _, clip := range layer.clips {
		if clip.weight <= 0 {
			continue
		}
		share := clip.weight / total
		copy(clipPoses, base)
		for n, w := range baseWeights {
			copy(clipWeights[n], w)
		}
		if clip.animation >= 0 {
			m.animations[clip.animation].poseAt(clip.time, clipPoses, clipWeights)
		}
		for n := range poses {
			c := clipPoses[n]
			if sum == 0 {
				poses[n] = nodePose{translation: c.translation.Scale(share), rotation: scaleTurn(c.rotation, share), scale: c.scale.Scale(share)}
			} else {
				p := &poses[n]
				p.translation = p.translation.Add(c.translation.Scale(share))
				p.scale = p.scale.Add(c.scale.Scale(share))
				r := c.rotation
				if p.rotation.dot(r) < 0 {
					r = quaternion{-r[0], -r[1], -r[2], -r[3]}
				}
				for k := range 4 {
					p.rotation[k] += r[k] * share
				}
			}
		}
		for n, w := range weights {
			for k := range w {
				w[k] += clipWeights[n][k] * share
			}
		}
		sum += share
	}
	for n := range poses {
		poses[n].rotation = poses[n].rotation.normalize()
	}
}

// shapesOn says whether an animation playing on layer moves the shape keys
// of node n.
func (a *Animator) shapesOn(layer *animationLayer, n int) bool {
	for _, clip := range layer.clips {
		if clip.animation < 0 {
			continue
		}
		for _, c := range a.model.animations[clip.animation].channels {
			if c.path == pathWeights && c.node == n {
				return true
			}
		}
	}
	return false
}

// growPoses returns *buffer at length n, making it again only when it is short.
func growPoses(buffer *[]nodePose, n int) []nodePose {
	if cap(*buffer) < n {
		*buffer = make([]nodePose, n)
	}
	*buffer = (*buffer)[:n]
	return *buffer
}

// copyWeights returns *buffer holding a copy of weights, node by node, with
// nil where weights has nil, making its slices only when they are short.
func copyWeights(buffer *[][]float32, weights [][]float32) [][]float32 {
	if len(*buffer) != len(weights) {
		*buffer = make([][]float32, len(weights))
	}
	out := *buffer
	for n, w := range weights {
		if w == nil {
			out[n] = nil
			continue
		}
		out[n] = append(out[n][:0], w...)
	}
	return out
}

func scaleTurn(q quaternion, by float32) quaternion {
	return quaternion{q[0] * by, q[1] * by, q[2] * by, q[3] * by}
}

// mixPose returns the pose by of the way from a to b.
func mixPose(a, b nodePose, by float32) nodePose {
	return nodePose{
		translation: a.translation.Lerp(b.translation, by),
		rotation:    slerp(a.rotation, b.rotation, by),
		scale:       a.scale.Lerp(b.scale, by),
	}
}

// restPose sets poses and weights to the model at rest.
func (m *Model) restPose(poses []nodePose, weights [][]float32) {
	for n, node := range m.nodes {
		poses[n] = node.rest
		if weights[n] != nil {
			copy(weights[n], node.weights)
		}
	}
}

// place works out where every node of p is in the model's space.
func (m *Model) place(p *modelPose) {
	for _, n := range m.order {
		node := &m.nodes[n]
		local := node.local
		if !node.byMatrix {
			local = p.locals[n].matrix()
		}
		if node.parent >= 0 {
			p.globals[n] = p.globals[node.parent].mul(local)
		} else {
			p.globals[n] = local
		}
	}
}

// bend works out the matrices that bend p's skins: each joint where it is
// now, times where the mesh was bound to it.
func (m *Model) bend(p *modelPose) {
	for s, skin := range m.skins {
		out := p.joints[s]
		for k, joint := range skin.joints {
			j := p.globals[joint].mul(skin.inverseBind[k])
			copy(out[k*16:], j[:])
		}
	}
}

// rootAt returns where the animation alone puts the root bone at time
// seconds, in the model's space, its parents at rest.
func (m *Model) rootAt(anim *modelAnimation, time float32) Vector3 {
	root := m.rootBone()
	position := m.nodes[root].rest.translation
	var value [3]float32
	for _, c := range anim.root {
		anim.channels[c].sample(time, value[:])
		position = Vector3{X: value[0], Y: value[1], Z: value[2]}
	}
	if parent := m.nodes[root].parent; parent >= 0 {
		return m.rest.globals[parent].transformPoint(position)
	}
	return position
}

// rootBone returns the node root motion follows: the top bone of the
// model's first skin, or -1 for a model without bones.
func (m *Model) rootBone() int {
	if len(m.skins) == 0 {
		return -1
	}
	return m.skins[m.rootSkin].root
}

func (m *Model) animationIndex(name string) int {
	for i, a := range m.animations {
		if a.name == name {
			return i
		}
	}
	return -1
}

// nodeIndex returns the first node called name, or -1.
func (m *Model) nodeIndex(name string) int {
	for i, n := range m.nodes {
		if n.name == name {
			return i
		}
	}
	return -1
}

// animationList names the model's animations, for a mistake's message.
func (m *Model) animationList() string {
	var names []string
	for _, a := range m.animations {
		names = append(names, a.name)
	}
	return nameList(names, "no animations: export them from Blender with Animation ticked")
}

// boneNames returns the names of the joints of the model's skins, each once.
func (m *Model) boneNames() []string {
	var names []string
	seen := map[int]bool{}
	for _, skin := range m.skins {
		for _, j := range skin.joints {
			if !seen[j] {
				seen[j] = true
				names = append(names, m.nodes[j].name)
			}
		}
	}
	return names
}

// boneList names the model's bones, or its nodes when it has no skeleton,
// for a mistake's message.
func (m *Model) boneList() string {
	names := m.boneNames()
	if len(names) == 0 {
		for _, n := range m.nodes {
			names = append(names, n.name)
		}
	}
	return nameList(names, "none")
}

// hasShapeKey says whether a mesh of the model has a shape key called name.
func (m *Model) hasShapeKey(name string) bool {
	for _, part := range m.parts {
		for _, key := range part.mesh.shapeKeys {
			if key == name {
				return true
			}
		}
	}
	return false
}

func (m *Model) shapeKeyList() string {
	var names []string
	seen := map[string]bool{}
	for _, part := range m.parts {
		for _, key := range part.mesh.shapeKeys {
			if !seen[key] {
				seen[key] = true
				names = append(names, key)
			}
		}
	}
	return nameList(names, "none")
}

// setShapeKey sets the shape key called name to weight in every mesh with
// one.
func (m *Model) setShapeKey(weights [][]float32, name string, weight float32) {
	for _, part := range m.parts {
		w := weights[part.node]
		for k, key := range part.mesh.shapeKeys {
			if key == name && k < len(w) {
				w[k] = weight
			}
		}
	}
}

// nameList quotes names for a message, or says none.
func nameList(names []string, none string) string {
	if len(names) == 0 {
		return none
	}
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(quoted, ", ")
}

// DrawAnimated draws the model of animator, posed as its animations have it
// now, placed by transform and tinted by tint, as [Screen.DrawModel] does.
func (s *Screen) DrawAnimated(animator *Animator, transform Transform, tint Color) {
	if !s.in3D("DrawAnimated") {
		return
	}
	if animator == nil {
		reportError(errors.New("golib: Screen.DrawAnimated got a nil Animator: make one with golib.NewAnimator"))
		return
	}
	model := animator.model
	model.mu.Lock()
	defer model.mu.Unlock()
	if !model.readyToDraw() {
		return
	}
	s.drawPose(model, animator.posedNow(), transform.matrix(), tint)
}
