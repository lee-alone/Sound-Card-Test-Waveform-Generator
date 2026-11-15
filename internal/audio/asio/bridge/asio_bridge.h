#ifndef ASIO_BRIDGE_H
#define ASIO_BRIDGE_H

#include "asio_types.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef void* AsioBridgeHandle;

// Bridge Lifecycle
AsioBridgeHandle asio_bridge_create(const char* clsid_str);
HRESULT asio_bridge_get_last_create_hresult(void);
void asio_bridge_destroy(AsioBridgeHandle handle);

// ASIO Methods
ASIOBool asio_bridge_init(AsioBridgeHandle handle, void* sys_handle);
void asio_bridge_get_driver_name(AsioBridgeHandle handle, char* name);
long asio_bridge_get_driver_version(AsioBridgeHandle handle);
void asio_bridge_get_error_message(AsioBridgeHandle handle, char* string);
ASIOError asio_bridge_start(AsioBridgeHandle handle);
ASIOError asio_bridge_stop(AsioBridgeHandle handle);
ASIOError asio_bridge_get_channels(AsioBridgeHandle handle, long* num_inputs, long* num_outputs);
ASIOError asio_bridge_get_latencies(AsioBridgeHandle handle, long* input_latency, long* output_latency);
ASIOError asio_bridge_get_buffer_size(AsioBridgeHandle handle, long* min_size, long* max_size, long* preferred_size, long* granularity);
ASIOError asio_bridge_can_sample_rate(AsioBridgeHandle handle, double sample_rate);
ASIOError asio_bridge_get_sample_rate(AsioBridgeHandle handle, double* sample_rate);
ASIOError asio_bridge_set_sample_rate(AsioBridgeHandle handle, double sample_rate);
ASIOError asio_bridge_get_clock_sources(AsioBridgeHandle handle, ASIOClockSource* clocks, long* num_sources);
ASIOError asio_bridge_set_clock_source(AsioBridgeHandle handle, long reference);
ASIOError asio_bridge_get_channel_info(AsioBridgeHandle handle, long channel, int is_input, ASIOChannelInfo* info);
ASIOError asio_bridge_create_buffers(AsioBridgeHandle handle, long num_channels, long buffer_size);
ASIOError asio_bridge_dispose_buffers(AsioBridgeHandle handle);
ASIOError asio_bridge_control_panel(AsioBridgeHandle handle);
ASIOError asio_bridge_future(AsioBridgeHandle handle, long selector, void* opt);
ASIOError asio_bridge_output_ready(AsioBridgeHandle handle);

// Thread-safe Synchronization & Buffers
HANDLE asio_bridge_get_wait_handle(AsioBridgeHandle handle);
long asio_bridge_get_active_buffer_index(AsioBridgeHandle handle);
void* asio_bridge_get_channel_buffer(AsioBridgeHandle handle, long channel, long buffer_index);
void asio_bridge_write_interleaved_pcm(AsioBridgeHandle handle, long buffer_index, const void* src, long num_frames, int bytes_per_sample, long num_channels);
void asio_bridge_write_planar_dsd(AsioBridgeHandle handle, long buffer_index, long channel, const void* src, long num_bytes);

#ifdef __cplusplus
}
#endif

#endif // ASIO_BRIDGE_H
