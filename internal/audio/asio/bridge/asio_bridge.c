#include "asio_bridge.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#define MAX_ASIO_CHANNELS 64

typedef struct AsioBridgeState {
    IASIO* driver;
    HANDLE hBufferSwitchEvent;
    volatile long activeBufferIndex;
    long numOutputChannels;
    long bufferSize;
    ASIOBufferInfo bufferInfos[MAX_ASIO_CHANNELS];
    ASIOCallbacks callbacks;
    int comInitialized;
} AsioBridgeState;

static AsioBridgeState* s_active_bridge = NULL;

// C static callbacks invoked by ASIO driver's real-time OS thread
static void asio_buffer_switch_callback(long doubleBufferIndex, ASIOBool directProcess) {
    (void)directProcess;
    if (s_active_bridge) {
        s_active_bridge->activeBufferIndex = doubleBufferIndex;
        if (s_active_bridge->hBufferSwitchEvent != NULL) {
            SetEvent(s_active_bridge->hBufferSwitchEvent);
        }
    }
}

static void asio_sample_rate_did_change(ASIOSampleRate sRate) {
    (void)sRate;
}

static long asio_message_callback(long selector, long value, void* message, double* opt) {
    (void)value;
    (void)message;
    (void)opt;
    // Return 0 for unhandled messages
    return 0;
}

static void* asio_buffer_switch_time_info(void* params, long doubleBufferIndex, ASIOBool directProcess) {
    asio_buffer_switch_callback(doubleBufferIndex, directProcess);
    return params;
}

AsioBridgeHandle asio_bridge_create(const char* clsid_str) {
    if (!clsid_str || strlen(clsid_str) == 0) {
        return NULL;
    }

    AsioBridgeState* state = (AsioBridgeState*)calloc(1, sizeof(AsioBridgeState));
    if (!state) {
        return NULL;
    }

    // Initialize COM on this thread
    HRESULT hr = CoInitializeEx(NULL, COINIT_MULTITHREADED);
    if (SUCCEEDED(hr)) {
        state->comInitialized = 1;
    } else if (hr == RPC_E_CHANGED_MODE) {
        state->comInitialized = 0; // Already initialized in another mode, still usable
    } else {
        free(state);
        return NULL;
    }

    // Convert string CLSID to GUID
    wchar_t wClsid[64] = {0};
    MultiByteToWideChar(CP_UTF8, 0, clsid_str, -1, wClsid, 63);

    CLSID clsid;
    hr = CLSIDFromString(wClsid, &clsid);
    if (FAILED(hr)) {
        if (state->comInitialized) CoUninitialize();
        free(state);
        return NULL;
    }

    // Create in-proc instance of ASIO driver
    IASIO* driver = NULL;
    hr = CoCreateInstance(&clsid, NULL, CLSCTX_INPROC_SERVER, &clsid, (void**)&driver);
    if (FAILED(hr) || !driver) {
        if (state->comInitialized) CoUninitialize();
        free(state);
        return NULL;
    }

    state->driver = driver;
    state->hBufferSwitchEvent = CreateEvent(NULL, FALSE, FALSE, NULL);
    state->activeBufferIndex = 0;

    // Setup callbacks
    state->callbacks.bufferSwitch = asio_buffer_switch_callback;
    state->callbacks.sampleRateDidChange = asio_sample_rate_did_change;
    state->callbacks.asioMessage = asio_message_callback;
    state->callbacks.bufferSwitchTimeInfo = asio_buffer_switch_time_info;

    s_active_bridge = state;
    return (AsioBridgeHandle)state;
}

void asio_bridge_destroy(AsioBridgeHandle handle) {
    if (!handle) return;
    AsioBridgeState* state = (AsioBridgeState*)handle;

    if (s_active_bridge == state) {
        s_active_bridge = NULL;
    }

    if (state->driver) {
        state->driver->lpVtbl->stop(state->driver);
        state->driver->lpVtbl->disposeBuffers(state->driver);
        state->driver->lpVtbl->Release(state->driver);
        state->driver = NULL;
    }

    if (state->hBufferSwitchEvent != NULL) {
        CloseHandle(state->hBufferSwitchEvent);
        state->hBufferSwitchEvent = NULL;
    }

    if (state->comInitialized) {
        CoUninitialize();
        state->comInitialized = 0;
    }

    free(state);
}

ASIOBool asio_bridge_init(AsioBridgeHandle handle, void* sys_handle) {
    if (!handle) return ASIOFalse;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->init(state->driver, sys_handle);
}

void asio_bridge_get_driver_name(AsioBridgeHandle handle, char* name) {
    if (!handle || !name) return;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    state->driver->lpVtbl->getDriverName(state->driver, name);
}

long asio_bridge_get_driver_version(AsioBridgeHandle handle) {
    if (!handle) return 0;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->getDriverVersion(state->driver);
}

void asio_bridge_get_error_message(AsioBridgeHandle handle, char* string) {
    if (!handle || !string) return;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    state->driver->lpVtbl->getErrorMessage(state->driver, string);
}

ASIOError asio_bridge_start(AsioBridgeHandle handle) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->start(state->driver);
}

ASIOError asio_bridge_stop(AsioBridgeHandle handle) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->stop(state->driver);
}

ASIOError asio_bridge_get_channels(AsioBridgeHandle handle, long* num_inputs, long* num_outputs) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->getChannels(state->driver, num_inputs, num_outputs);
}

ASIOError asio_bridge_get_latencies(AsioBridgeHandle handle, long* input_latency, long* output_latency) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->getLatencies(state->driver, input_latency, output_latency);
}

ASIOError asio_bridge_get_buffer_size(AsioBridgeHandle handle, long* min_size, long* max_size, long* preferred_size, long* granularity) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->getBufferSize(state->driver, min_size, max_size, preferred_size, granularity);
}

ASIOError asio_bridge_can_sample_rate(AsioBridgeHandle handle, double sample_rate) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->canSampleRate(state->driver, sample_rate);
}

ASIOError asio_bridge_get_sample_rate(AsioBridgeHandle handle, double* sample_rate) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->getSampleRate(state->driver, sample_rate);
}

ASIOError asio_bridge_set_sample_rate(AsioBridgeHandle handle, double sample_rate) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->setSampleRate(state->driver, sample_rate);
}

ASIOError asio_bridge_get_clock_sources(AsioBridgeHandle handle, ASIOClockSource* clocks, long* num_sources) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->getClockSources(state->driver, clocks, num_sources);
}

ASIOError asio_bridge_set_clock_source(AsioBridgeHandle handle, long reference) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->setClockSource(state->driver, reference);
}

ASIOError asio_bridge_get_channel_info(AsioBridgeHandle handle, long channel, int is_input, ASIOChannelInfo* info) {
    if (!handle || !info) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    info->channel = channel;
    info->isInput = is_input ? ASIOTrue : ASIOFalse;
    return state->driver->lpVtbl->getChannelInfo(state->driver, info);
}

ASIOError asio_bridge_create_buffers(AsioBridgeHandle handle, long num_channels, long buffer_size) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    if (num_channels <= 0 || num_channels > MAX_ASIO_CHANNELS) {
        return ASE_InvalidParameter;
    }

    state->numOutputChannels = num_channels;
    state->bufferSize = buffer_size;

    for (long i = 0; i < num_channels; i++) {
        state->bufferInfos[i].isInput = ASIOFalse;
        state->bufferInfos[i].channelNum = i;
        state->bufferInfos[i].buffers[0] = NULL;
        state->bufferInfos[i].buffers[1] = NULL;
    }

    return state->driver->lpVtbl->createBuffers(state->driver, state->bufferInfos, num_channels, buffer_size, &state->callbacks);
}

ASIOError asio_bridge_dispose_buffers(AsioBridgeHandle handle) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->disposeBuffers(state->driver);
}

ASIOError asio_bridge_control_panel(AsioBridgeHandle handle) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->controlPanel(state->driver);
}

ASIOError asio_bridge_future(AsioBridgeHandle handle, long selector, void* opt) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->future(state->driver, selector, opt);
}

ASIOError asio_bridge_output_ready(AsioBridgeHandle handle) {
    if (!handle) return ASE_NotPresent;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->driver->lpVtbl->outputReady(state->driver);
}

HANDLE asio_bridge_get_wait_handle(AsioBridgeHandle handle) {
    if (!handle) return NULL;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->hBufferSwitchEvent;
}

long asio_bridge_get_active_buffer_index(AsioBridgeHandle handle) {
    if (!handle) return 0;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    return state->activeBufferIndex;
}

void* asio_bridge_get_channel_buffer(AsioBridgeHandle handle, long channel, long buffer_index) {
    if (!handle) return NULL;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    if (channel < 0 || channel >= state->numOutputChannels || (buffer_index != 0 && buffer_index != 1)) {
        return NULL;
    }
    return state->bufferInfos[channel].buffers[buffer_index];
}

void asio_bridge_write_interleaved_pcm(AsioBridgeHandle handle, long buffer_index, const void* src, long num_frames, int bytes_per_sample, long num_channels) {
    if (!handle || !src || (buffer_index != 0 && buffer_index != 1)) return;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    if (num_channels > state->numOutputChannels) num_channels = state->numOutputChannels;

    const unsigned char* src_bytes = (const unsigned char*)src;
    int frame_stride = bytes_per_sample * (int)num_channels;

    for (long ch = 0; ch < num_channels; ch++) {
        unsigned char* dst = (unsigned char*)state->bufferInfos[ch].buffers[buffer_index];
        if (!dst) continue;

        const unsigned char* s = src_bytes + (ch * bytes_per_sample);
        for (long f = 0; f < num_frames; f++) {
            memcpy(dst + (f * bytes_per_sample), s, bytes_per_sample);
            s += frame_stride;
        }
    }
}

void asio_bridge_write_planar_dsd(AsioBridgeHandle handle, long buffer_index, long channel, const void* src, long num_bytes) {
    if (!handle || !src || (buffer_index != 0 && buffer_index != 1)) return;
    AsioBridgeState* state = (AsioBridgeState*)handle;
    if (channel < 0 || channel >= state->numOutputChannels) return;

    void* dst = state->bufferInfos[channel].buffers[buffer_index];
    if (dst) {
        memcpy(dst, src, (size_t)num_bytes);
    }
}
