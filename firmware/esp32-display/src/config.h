#pragma once

#include <Arduino.h>

namespace config {

constexpr uint8_t kSdaPin = 21;
constexpr uint8_t kSclPin = 22;
constexpr uint8_t kDisplayAddress = 0x3C;
constexpr uint8_t kDisplayWidth = 128;
constexpr uint8_t kDisplayHeight = 64;

constexpr uint8_t kStartButtonPin = 27;
constexpr uint8_t kResetButtonPin = 26;
constexpr uint8_t kSkipButtonPin = 32;

constexpr uint8_t kButtonDebounceMs = 35;
constexpr uint8_t kButtonLongPressMs = 700;

constexpr uint16_t kDisplayRefreshMs = 120;
constexpr uint16_t kSocketPollMs = 4;

constexpr uint32_t kSocketReconnectMs = 4000;
constexpr uint32_t kWifiRetryMs = 8000;
constexpr uint32_t kPingIntervalMs = 20000;

constexpr size_t kTaskTitleLength = 40;
constexpr size_t kClientNameLength = 32;

constexpr uint32_t kStaleAfterMs = 12000;

} config
