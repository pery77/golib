package main

import "golib"

// The models, from the FBX pack made from Kenney's Platformer Kit (see
// assets/ATTRIBUTION.md), made once. The astronaut comes with its skin and no
// animation, and each animation is a file of its own, read into it by name:
// "models/walk.fbx" plays as "walk".
var (
	astronautModel = golib.NewModel("models/astronaut.fbx").AddAnimations(
		"models/idle.fbx", "models/walk.fbx", "models/sprint.fbx", "models/jump.fbx", "models/fall.fbx",
		"models/punch.fbx", "models/die.fbx", "models/emote-yes.fbx", "models/pick-up.fbx",
	)
	blockModel   = golib.NewModel("models/block-grass-large.fbx")
	coinModel    = golib.NewModel("models/coin-gold.fbx")
	flagModel    = golib.NewModel("models/flag.fbx")
	rocksModel   = golib.NewModel("models/rocks.fbx")
	treeModel    = golib.NewModel("models/tree.fbx")
	pineModel    = golib.NewModel("models/tree-pine.fbx")
	flowersModel = golib.NewModel("models/flowers.fbx")
)

// clip is one of the astronaut's animations.
type clip struct{ name string }

// length is how long the clip lasts at its own speed. The pack's files were
// exported from Blender with its whole timeline, frames 1 to 250, holding
// their last pose after their moves; golib ends each where its moves end.
func (c clip) length() float32 { return astronautModel.AnimationLength(c.name) }

var (
	clipIdle   = clip{"idle"}
	clipWalk   = clip{"walk"}
	clipSprint = clip{"sprint"}
	clipJump   = clip{"jump"}
	clipFall   = clip{"fall"}
	clipPunch  = clip{"punch"}
	clipDie    = clip{"die"}
	clipYes    = clip{"emote-yes"}
	clipPickUp = clip{"pick-up"}
)

// The astronaut's bones, as Kenney named them.
const boneTorso = "torso" // the body, the head and both arms: punches and pick-ups play here, over the legs

// loop plays c on the whole body as a loop, at speed (1 when zero), fading in
// from what played. Call it in every update, as Animator.Play.
func loop(a *golib.Animator, c clip, speed, fade float32) {
	a.PlayWith(c.name, golib.AnimationOptions{Speed: speed, Fade: fade})
}

// playOnce plays c once on the whole body and holds its last pose.
func playOnce(a *golib.Animator, c clip, fade float32) {
	a.PlayWith(c.name, golib.AnimationOptions{Once: true, Fade: fade})
}

// layerOver plays c once on the torso, over what the legs do; the layer
// fades out at its end.
func layerOver(a *golib.Animator, c clip, speed float32) {
	a.PlayWith(c.name, golib.AnimationOptions{Bone: boneTorso, Once: true, Restart: true, Fade: 0.06, Speed: speed})
}
