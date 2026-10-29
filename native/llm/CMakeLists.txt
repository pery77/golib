# golibllm: llama.cpp, behind a small C interface that package golib calls
# through purego, for language models that run inside a game. Continuous
# integration builds it for each platform (see .github/workflows/llm.yml);
# nobody making or playing a game needs a C++ compiler. See native/README.md.
#
# GOLIB_LLM_BACKENDS_APART builds llama.cpp's compute backends as libraries
# of their own, loaded when the game starts: one CPU backend per processor
# generation, so each computer gets the fastest its processor runs, and
# Vulkan for graphics cards, which a computer without Vulkan simply skips.
# Without it, as on macOS, everything is inside golibllm: Metal on Apple
# silicon, the CPU elsewhere.
cmake_minimum_required(VERSION 3.20)
set(CMAKE_OSX_DEPLOYMENT_TARGET "13.3" CACHE STRING "")
project(golibllm LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)
if(NOT CMAKE_BUILD_TYPE)
	set(CMAKE_BUILD_TYPE Release)
endif()

option(GOLIB_LLM_BACKENDS_APART "llama.cpp's backends as libraries of their own" OFF)
option(GOLIB_LLM_VULKAN "the Vulkan backend, for graphics cards" OFF)

include(FetchContent)
# llama.cpp, at a fixed build: its source is fetched at this commit, and its
# notice travels with every game that uses a language model.
FetchContent_Declare(
	llama
	GIT_REPOSITORY https://github.com/ggml-org/llama.cpp.git
	GIT_TAG cf24317bea431b807dc93319d74c61bed6e20aff # b11972
)
set(LLAMA_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_TOOLS OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_SERVER OFF CACHE BOOL "" FORCE)
set(LLAMA_BUILD_COMMON OFF CACHE BOOL "" FORCE)
set(LLAMA_CURL OFF CACHE BOOL "" FORCE)
set(LLAMA_OPENSSL OFF CACHE BOOL "" FORCE)
set(GGML_NATIVE OFF CACHE BOOL "" FORCE)
set(GGML_OPENMP OFF CACHE BOOL "" FORCE)
if(GOLIB_LLM_BACKENDS_APART)
	set(BUILD_SHARED_LIBS ON CACHE BOOL "" FORCE)
	set(GGML_BACKEND_DL ON CACHE BOOL "" FORCE)
	set(GGML_CPU_ALL_VARIANTS ON CACHE BOOL "" FORCE)
else()
	set(BUILD_SHARED_LIBS OFF CACHE BOOL "" FORCE)
endif()
set(GGML_VULKAN ${GOLIB_LLM_VULKAN} CACHE BOOL "" FORCE)
if(APPLE AND CMAKE_OSX_ARCHITECTURES STREQUAL "arm64" OR APPLE AND CMAKE_SYSTEM_PROCESSOR STREQUAL "arm64")
	set(GGML_METAL ON CACHE BOOL "" FORCE)
	set(GGML_METAL_EMBED_LIBRARY ON CACHE BOOL "" FORCE)
else()
	set(GGML_METAL OFF CACHE BOOL "" FORCE)
	if(NOT GOLIB_LLM_BACKENDS_APART AND CMAKE_SYSTEM_PROCESSOR MATCHES "x86_64|AMD64")
		# One build for every x86 processor of the last ten years: AVX2.
		set(GGML_AVX2 ON CACHE BOOL "" FORCE)
		set(GGML_FMA ON CACHE BOOL "" FORCE)
		set(GGML_F16C ON CACHE BOOL "" FORCE)
	endif()
endif()
# On Windows, with the backends apart, every library shares one C++ runtime,
# the DLLs that the workflow puts beside them, so players need nothing
# installed: llama.dll reads files that ggml-base.dll opens, and a FILE* from
# a runtime inside one library is an invalid parameter to the runtime inside
# another, which ends the game. A single golibllm keeps the runtime inside.
if(GOLIB_LLM_BACKENDS_APART)
	set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>DLL")
else()
	set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")
endif()
FetchContent_MakeAvailable(llama)

add_library(golibllm SHARED golib_llm.cpp)
target_link_libraries(golibllm PRIVATE llama ggml)
set_target_properties(golibllm PROPERTIES
	CXX_VISIBILITY_PRESET hidden
	VISIBILITY_INLINES_HIDDEN ON
	PREFIX ""
	OUTPUT_NAME "golibllm")
if(APPLE)
	set_target_properties(golibllm PROPERTIES SUFFIX ".dylib" BUILD_RPATH "@loader_path" INSTALL_RPATH "@loader_path")
elseif(UNIX)
	set_target_properties(golibllm PROPERTIES BUILD_RPATH "$ORIGIN" INSTALL_RPATH "$ORIGIN")
	target_link_options(golibllm PRIVATE -static-libstdc++ -static-libgcc)
endif()
