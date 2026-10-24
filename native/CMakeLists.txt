# GoLib's native library: Jolt Physics, behind a small C interface that
# package golib calls through purego. Continuous integration builds it for
# each platform (see .github/workflows/native.yml); nobody making or playing
# a game needs a C++ compiler. See native/README.md.
cmake_minimum_required(VERSION 3.20)
# The oldest macOS the library runs on, as old as GoLib's games reach.
set(CMAKE_OSX_DEPLOYMENT_TARGET "11.0" CACHE STRING "")
project(golibnative LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)
if(NOT CMAKE_BUILD_TYPE)
	set(CMAKE_BUILD_TYPE Release)
endif()

include(FetchContent)
# Jolt, at a fixed release: its source is fetched at this commit, and its
# notice travels with every game that uses rigid bodies.
FetchContent_Declare(
	JoltPhysics
	GIT_REPOSITORY https://github.com/jrouwe/JoltPhysics.git
	GIT_TAG e77f175595e64cb44218cc9d9d56fc365ad0e36a # v5.6.0
	SOURCE_SUBDIR Build
)
# Instructions any 64-bit x86 processor of the last fifteen years has, and
# none newer, so a game runs on the players' older computers too.
set(USE_SSE4_1 ON CACHE BOOL "" FORCE)
set(USE_SSE4_2 ON CACHE BOOL "" FORCE)
set(USE_AVX OFF CACHE BOOL "" FORCE)
set(USE_AVX2 OFF CACHE BOOL "" FORCE)
set(USE_AVX512 OFF CACHE BOOL "" FORCE)
set(USE_LZCNT OFF CACHE BOOL "" FORCE)
set(USE_TZCNT OFF CACHE BOOL "" FORCE)
set(USE_F16C OFF CACHE BOOL "" FORCE)
set(USE_FMADD OFF CACHE BOOL "" FORCE)
set(DOUBLE_PRECISION OFF CACHE BOOL "" FORCE)
set(CROSS_PLATFORM_DETERMINISTIC OFF CACHE BOOL "" FORCE)
set(INTERPROCEDURAL_OPTIMIZATION OFF CACHE BOOL "" FORCE)
set(ENABLE_ALL_WARNINGS OFF CACHE BOOL "" FORCE)
set(TARGET_UNIT_TESTS OFF CACHE BOOL "" FORCE)
set(TARGET_HELLO_WORLD OFF CACHE BOOL "" FORCE)
set(TARGET_PERFORMANCE_TEST OFF CACHE BOOL "" FORCE)
set(TARGET_SAMPLES OFF CACHE BOOL "" FORCE)
set(TARGET_VIEWER OFF CACHE BOOL "" FORCE)
set(USE_STATIC_MSVC_RUNTIME_LIBRARY ON CACHE BOOL "" FORCE)
# The same C++ runtime, inside the library, for golib_physics.cpp as Jolt
# has, or Visual Studio's linker refuses to put them together.
set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
FetchContent_MakeAvailable(JoltPhysics)

add_library(golibnative SHARED golib_physics.cpp)
target_link_libraries(golibnative PRIVATE Jolt)
target_include_directories(golibnative PRIVATE ${JoltPhysics_SOURCE_DIR}/..)
set_target_properties(golibnative PROPERTIES
	CXX_VISIBILITY_PRESET hidden
	VISIBILITY_INLINES_HIDDEN ON
	PREFIX ""
	OUTPUT_NAME "golibnative")
if(APPLE)
	set_target_properties(golibnative PROPERTIES SUFFIX ".dylib")
elseif(UNIX)
	# The C++ runtime goes inside, so the library loads on Linux systems
	# whose libstdc++ is older than the one it was built with.
	target_link_options(golibnative PRIVATE -static-libstdc++ -static-libgcc)
endif()
