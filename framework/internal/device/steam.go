package device

// Steam in the contract: Valve's Steamworks API, a library of Valve's that
// golib copies next to a game built with the build tag steam, called through
// purego (steam_desktop.go). Without the tag, and in a browser, the same
// functions do nothing (steam_off.go), so a game built without it neither
// carries nor needs the library.
//
// The names below are Steamworks' flat C API of SDK 1.64, whose interfaces
// carry their versions in their names. A newer SDK keeps the old versions
// working; when GoLib moves to new ones, these change with them (see
// docs/tooling.md#steamworks).
const (
	steamUserStatsInterface = "SteamAPI_SteamUserStats_v013"
	steamAppsInterface      = "SteamAPI_SteamApps_v009"
	steamUtilsInterface     = "SteamAPI_SteamUtils_v010"
)

// The results of SteamAPI_InitFlat, ESteamAPIInitResult.
const (
	steamInitOK              = 0
	steamInitFailedGeneric   = 1 // some other failure
	steamInitNoSteamClient   = 2 // Steam isn't running, or isn't logged in
	steamInitVersionMismatch = 3 // the Steam client is older than the library
)
