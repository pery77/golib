// golibllm: llama.cpp behind a small C interface, for package golib's
// language models (framework/languagemodel.go), called through purego.
//
// Every argument is a pointer or an integer, and floats go through pointers,
// because purego's SyscallN passes integers only, and llama.cpp's own
// interface takes and returns structs by value, which purego can't on every
// platform.
//
// A model keeps one conversation's context: each reply's prompt is the
// whole conversation, and the part of it the context already holds, from
// the start, isn't read again, so a long system prompt is read once.

#include <algorithm>
#include <cstdio>
#include <cstring>
#include <string>
#include <vector>

#include "ggml-backend.h"
#include "llama.h"

#if defined(_WIN32)
#define GOLIB_EXPORT extern "C" __declspec(dllexport)
#else
#define GOLIB_EXPORT extern "C" __attribute__((visibility("default")))
#endif

// The interface's version: framework/internal/device/raylib_llm.go checks it.
static const int golib_llm_interface = 1;

namespace {

struct model {
	llama_model *model = nullptr;
	llama_context *ctx = nullptr;
	const llama_vocab *vocab = nullptr;
	llama_sampler *sampler = nullptr;
	std::vector<llama_token> cached; // the tokens the context holds, in order
	int generated = 0;
	int max_tokens = 0;
	bool done = true;
	std::string pending; // bytes of a character split between tokens
};

void copy_text(const std::string &text, char *out, int32_t len) {
	if (out == nullptr || len <= 0) {
		return;
	}
	size_t n = std::min(text.size(), size_t(len - 1));
	memcpy(out, text.data(), n);
	out[n] = 0;
}

void quiet(ggml_log_level level, const char *text, void *) {
	if (level == GGML_LOG_LEVEL_ERROR) {
		fputs(text, stderr);
	}
}

bool backend_ready = false;

// decode feeds tokens to the context, in batches it can take.
bool decode(model *m, const llama_token *tokens, int n) {
	int batch = int(llama_n_batch(m->ctx));
	for (int i = 0; i < n; i += batch) {
		int count = std::min(batch, n - i);
		if (llama_decode(m->ctx, llama_batch_get_one(const_cast<llama_token *>(tokens + i), count)) != 0) {
			return false;
		}
		m->cached.insert(m->cached.end(), tokens + i, tokens + i + count);
	}
	return true;
}

// complete_utf8 returns how many bytes of s end on a whole UTF-8 character.
size_t complete_utf8(const std::string &s) {
	size_t i = s.size();
	size_t back = 0;
	while (i > 0 && back < 4) {
		unsigned char c = s[i - 1];
		if ((c & 0xC0) != 0x80) {
			size_t need = c < 0x80 ? 1 : (c >> 5) == 0x6 ? 2 : (c >> 4) == 0xE ? 3 : (c >> 3) == 0x1E ? 4 : 1;
			return back + 1 >= need ? s.size() : i - 1;
		}
		i--;
		back++;
	}
	return s.size();
}

} // namespace

GOLIB_EXPORT int32_t golib_llm_version() { return golib_llm_interface; }

// golib_llm_backends loads the compute backends kept as libraries of their
// own beside golibllm (the CPU for each processor generation, Vulkan), from
// dir. Builds with the backends inside need nothing loaded: it does nothing
// there but initialize llama.cpp.
GOLIB_EXPORT void golib_llm_backends(const char *dir) {
	if (backend_ready) {
		return;
	}
	backend_ready = true;
	llama_log_set(quiet, nullptr);
	if (dir != nullptr && dir[0] != 0) {
		ggml_backend_load_all_from_path(dir);
	}
	llama_backend_init();
}

// golib_llm_load reads a GGUF model. settings holds 3 integers: the
// context's size in tokens, the layers to put on the graphics card (0 for
// none, a large number for all), and the threads (0 for the default). It
// returns the model, or null with the reason in err.
GOLIB_EXPORT void *golib_llm_load(const char *path, const int32_t *settings, char *err, int32_t errlen) {
	golib_llm_backends(nullptr);
	llama_model_params mp = llama_model_default_params();
	mp.n_gpu_layers = settings[1];
	llama_model *lm = llama_model_load_from_file(path, mp);
	if (lm == nullptr) {
		copy_text(std::string("llama.cpp could not read the model ") + path, err, errlen);
		return nullptr;
	}
	llama_context_params cp = llama_context_default_params();
	cp.n_ctx = uint32_t(settings[0]);
	cp.n_batch = 512;
	if (settings[2] > 0) {
		cp.n_threads = settings[2];
		cp.n_threads_batch = settings[2];
	}
	llama_context *ctx = llama_init_from_model(lm, cp);
	if (ctx == nullptr) {
		llama_model_free(lm);
		copy_text("llama.cpp could not make a context for the model: too little memory for its context size?", err, errlen);
		return nullptr;
	}
	model *m = new model;
	m->model = lm;
	m->ctx = ctx;
	m->vocab = llama_model_get_vocab(lm);
	return m;
}

GOLIB_EXPORT void golib_llm_free(void *p) {
	model *m = static_cast<model *>(p);
	if (m == nullptr) {
		return;
	}
	if (m->sampler != nullptr) {
		llama_sampler_free(m->sampler);
	}
	llama_free(m->ctx);
	llama_model_free(m->model);
	delete m;
}

// golib_llm_template writes the conversation, n messages of roles and
// texts, in the model's own chat format, ending where the assistant's reply
// starts, into out. It returns the bytes it needs, which may be more than
// outlen, or -1 when the model's format is one llama.cpp doesn't know and
// ChatML doesn't do either.
GOLIB_EXPORT int32_t golib_llm_template(void *p, const char *const *roles, const char *const *texts, int32_t n, char *out, int32_t outlen) {
	model *m = static_cast<model *>(p);
	std::vector<llama_chat_message> chat(size_t(std::max(n, 0)));
	for (int32_t i = 0; i < n; i++) {
		chat[size_t(i)] = {roles[i], texts[i]};
	}
	const char *tmpl = llama_model_chat_template(m->model, nullptr);
	int32_t len = llama_chat_apply_template(tmpl, chat.data(), chat.size(), true, out, outlen);
	if (len < 0) {
		len = llama_chat_apply_template("chatml", chat.data(), chat.size(), true, out, outlen);
	}
	return len;
}

// golib_llm_start begins a reply to prompt, a whole conversation in the
// model's format. sampling holds 6 floats: temperature, top-p, min-p, the
// repeat penalty, the seed and the most tokens to write. It returns 0, or
// -1 with the reason in err.
GOLIB_EXPORT int32_t golib_llm_start(void *p, const char *prompt, const float *sampling, char *err, int32_t errlen) {
	model *m = static_cast<model *>(p);
	int len = int(strlen(prompt));
	int n = -llama_tokenize(m->vocab, prompt, len, nullptr, 0, true, true);
	std::vector<llama_token> tokens(size_t(std::max(n, 1)));
	if (llama_tokenize(m->vocab, prompt, len, tokens.data(), int32_t(tokens.size()), true, true) < 0) {
		copy_text("llama.cpp could not read the prompt into tokens", err, errlen);
		return -1;
	}
	tokens.resize(size_t(n));
	int max_tokens = int(sampling[5]);
	int n_ctx = int(llama_n_ctx(m->ctx));
	if (n + max_tokens > n_ctx) {
		copy_text("the conversation is longer than the model's context: keep fewer messages, or make the context larger", err, errlen);
		return -1;
	}
	// Keep what the context has of this prompt, from its start, but read its
	// last token again, so there are logits to sample from.
	size_t keep = 0;
	while (keep < m->cached.size() && keep < tokens.size() && m->cached[keep] == tokens[keep]) {
		keep++;
	}
	keep = std::min(keep, tokens.size() - 1);
	llama_memory_seq_rm(llama_get_memory(m->ctx), 0, llama_pos(keep), -1);
	m->cached.resize(keep);
	if (!decode(m, tokens.data() + keep, int(tokens.size() - keep))) {
		llama_memory_seq_rm(llama_get_memory(m->ctx), -1, -1, -1);
		m->cached.clear();
		copy_text("llama.cpp could not read the prompt", err, errlen);
		return -1;
	}
	if (m->sampler != nullptr) {
		llama_sampler_free(m->sampler);
	}
	llama_sampler *s = llama_sampler_chain_init(llama_sampler_chain_default_params());
	llama_sampler_chain_add(s, llama_sampler_init_top_k(40));
	llama_sampler_chain_add(s, llama_sampler_init_penalties(llama_vocab_n_tokens(m->vocab), 64, sampling[3], 0, 0));
	llama_sampler_chain_add(s, llama_sampler_init_top_p(sampling[1], 1));
	llama_sampler_chain_add(s, llama_sampler_init_min_p(sampling[2], 1));
	if (sampling[0] <= 0) {
		llama_sampler_chain_add(s, llama_sampler_init_greedy());
	} else {
		llama_sampler_chain_add(s, llama_sampler_init_temp(sampling[0]));
		llama_sampler_chain_add(s, llama_sampler_init_dist(uint32_t(sampling[4])));
	}
	m->sampler = s;
	m->generated = 0;
	m->max_tokens = max_tokens;
	m->done = false;
	m->pending.clear();
	return 0;
}

// golib_llm_next writes the reply's next piece of text into out and returns
// its length: 0 at the end of the reply, -1 on an error. A piece is whole
// characters; it can be empty while a character is split between tokens.
GOLIB_EXPORT int32_t golib_llm_next(void *p, char *out, int32_t outlen) {
	model *m = static_cast<model *>(p);
	while (!m->done) {
		if (m->generated >= m->max_tokens) {
			m->done = true;
			break;
		}
		llama_token t = llama_sampler_sample(m->sampler, m->ctx, -1);
		if (llama_vocab_is_eog(m->vocab, t)) {
			m->done = true;
			break;
		}
		char buf[256];
		int n = llama_token_to_piece(m->vocab, t, buf, sizeof buf, 0, false);
		if (n < 0) {
			m->done = true;
			return -1;
		}
		m->generated++;
		if (!decode(m, &t, 1)) {
			m->done = true;
			return -1;
		}
		m->pending.append(buf, size_t(n));
		size_t whole = complete_utf8(m->pending);
		if (whole == 0) {
			continue;
		}
		std::string piece = m->pending.substr(0, whole);
		m->pending.erase(0, whole);
		if (int32_t(piece.size()) >= outlen) {
			piece.resize(size_t(outlen - 1));
		}
		copy_text(piece, out, outlen);
		return int32_t(piece.size());
	}
	if (!m->pending.empty()) {
		std::string piece;
		piece.swap(m->pending);
		copy_text(piece, out, outlen);
		return int32_t(std::min(piece.size(), size_t(outlen - 1)));
	}
	return 0;
}

// golib_llm_stop ends the reply being written.
GOLIB_EXPORT void golib_llm_stop(void *p) {
	static_cast<model *>(p)->done = true;
}
