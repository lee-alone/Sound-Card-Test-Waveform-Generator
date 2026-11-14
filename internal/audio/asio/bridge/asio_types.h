#ifndef ASIO_TYPES_H
#define ASIO_TYPES_H

#include <windows.h>
#include <ole2.h>

#ifdef __cplusplus
extern "C" {
#endif

// ASIO Error Codes
typedef long ASIOError;
#define ASE_OK                   0
#define ASE_SUCCESS              0x3f484c20
#define ASE_NotPresent          -1000
#define ASE_HWMalfunction       -999
#define ASE_InvalidParameter    -998
#define ASE_InvalidMode         -997
#define ASE_SPNotAdvancing      -996
#define ASE_NoClock             -995
#define ASE_NoMemory            -994

// ASIO Boolean
typedef long ASIOBool;
#define ASIOFalse 0
#define ASIOTrue  1

// ASIO Sample Rates
typedef double ASIOSampleRate;

// ASIO Sample Types
typedef long ASIOSampleType;
#define ASIOSTInt16MSB           0
#define ASIOSTInt24MSB           1
#define ASIOSTInt32MSB           2
#define ASIOSTFloat32MSB         3
#define ASIOSTFloat64MSB         4
#define ASIOSTInt32MSB16         8
#define ASIOSTInt32MSB18         9
#define ASIOSTInt32MSB20         10
#define ASIOSTInt32MSB24         11
#define ASIOSTInt16LSB           16
#define ASIOSTInt24LSB           17
#define ASIOSTInt32LSB           18
#define ASIOSTFloat32LSB         19
#define ASIOSTFloat64LSB         20
#define ASIOSTInt32LSB16         24
#define ASIOSTInt32LSB18         25
#define ASIOSTInt32LSB20         26
#define ASIOSTInt32LSB24         27
#define ASIOSTDSDInt8LSB1        32
#define ASIOSTDSDInt8MSB1        33
#define ASIOSTDSDInt8NER8        34

// 64-bit int structures
typedef struct ASIOSamples {
    unsigned long lo;
    unsigned long hi;
} ASIOSamples;

typedef struct ASIOTimeStamp {
    unsigned long lo;
    unsigned long hi;
} ASIOTimeStamp;

// Channel Info
typedef struct ASIOChannelInfo {
    long channel;
    ASIOBool isInput;
    ASIOBool isActive;
    long channelGroup;
    ASIOSampleType type;
    char name[32];
} ASIOChannelInfo;

// Buffer Info
typedef struct ASIOBufferInfo {
    ASIOBool isInput;
    long channelNum;
    void *buffers[2];
} ASIOBufferInfo;

// Clock Source
typedef struct ASIOClockSource {
    long index;
    long associatedChannel;
    long associatedGroup;
    ASIOBool isCurrentSource;
    char name[128];
} ASIOClockSource;

// Callbacks
typedef struct ASIOCallbacks {
    void (*bufferSwitch)(long doubleBufferIndex, ASIOBool directProcess);
    void (*sampleRateDidChange)(ASIOSampleRate sRate);
    long (*asioMessage)(long selector, long value, void* message, double* opt);
    void* (*bufferSwitchTimeInfo)(void* params, long doubleBufferIndex, ASIOBool directProcess);
} ASIOCallbacks;

// Future Selectors
#define kAsioEnableTimeCodeRead  1
#define kAsioDisableTimeCodeRead 2
#define kAsioSetInputMonitor     3
#define kAsioTransport           4
#define kAsioSetInputGain        5
#define kAsioGetInputMeter       6
#define kAsioSetIoFormat         7
#define kAsioGetIoFormat         8
#define kAsioCanDoIoFormat       9

// DSD IO Format for future(kAsioSetIoFormat)
typedef long ASIOIoFormatType;
#define kASIOFormatInvalid      -1
#define kASIOPCMFormat           0
#define kASIODSDFormat           1

typedef struct ASIOIoFormat {
    ASIOIoFormatType FormatType;
    char future[512];
} ASIOIoFormat;

// Forward declaration of IASIO COM Interface
typedef struct IASIO IASIO;

typedef struct IASIOVtbl {
    // IUnknown
    HRESULT (STDMETHODCALLTYPE *QueryInterface)(IASIO *this, REFIID riid, void **ppvObject);
    ULONG (STDMETHODCALLTYPE *AddRef)(IASIO *this);
    ULONG (STDMETHODCALLTYPE *Release)(IASIO *this);

    // IASIO Methods
    ASIOBool (STDMETHODCALLTYPE *init)(IASIO *this, void *sysHandle);
    void (STDMETHODCALLTYPE *getDriverName)(IASIO *this, char *name);
    long (STDMETHODCALLTYPE *getDriverVersion)(IASIO *this);
    void (STDMETHODCALLTYPE *getErrorMessage)(IASIO *this, char *string);
    ASIOError (STDMETHODCALLTYPE *start)(IASIO *this);
    ASIOError (STDMETHODCALLTYPE *stop)(IASIO *this);
    ASIOError (STDMETHODCALLTYPE *getChannels)(IASIO *this, long *numInputChannels, long *numOutputChannels);
    ASIOError (STDMETHODCALLTYPE *getLatencies)(IASIO *this, long *inputLatency, long *outputLatency);
    ASIOError (STDMETHODCALLTYPE *getBufferSize)(IASIO *this, long *minSize, long *maxSize, long *preferredSize, long *granularity);
    ASIOError (STDMETHODCALLTYPE *canSampleRate)(IASIO *this, ASIOSampleRate sampleRate);
    ASIOError (STDMETHODCALLTYPE *getSampleRate)(IASIO *this, ASIOSampleRate *sampleRate);
    ASIOError (STDMETHODCALLTYPE *setSampleRate)(IASIO *this, ASIOSampleRate sampleRate);
    ASIOError (STDMETHODCALLTYPE *getClockSources)(IASIO *this, ASIOClockSource *clocks, long *numSources);
    ASIOError (STDMETHODCALLTYPE *setClockSource)(IASIO *this, long reference);
    ASIOError (STDMETHODCALLTYPE *getSamplePosition)(IASIO *this, ASIOSamples *sPos, ASIOTimeStamp *tStamp);
    ASIOError (STDMETHODCALLTYPE *getChannelInfo)(IASIO *this, ASIOChannelInfo *info);
    ASIOError (STDMETHODCALLTYPE *createBuffers)(IASIO *this, ASIOBufferInfo *bufferInfos, long numChannels, long bufferSize, ASIOCallbacks *callbacks);
    ASIOError (STDMETHODCALLTYPE *disposeBuffers)(IASIO *this);
    ASIOError (STDMETHODCALLTYPE *controlPanel)(IASIO *this);
    ASIOError (STDMETHODCALLTYPE *future)(IASIO *this, long selector, void *opt);
    ASIOError (STDMETHODCALLTYPE *outputReady)(IASIO *this);
} IASIOVtbl;

struct IASIO {
    const IASIOVtbl *lpVtbl;
};

#ifdef __cplusplus
}
#endif

#endif // ASIO_TYPES_H
