#pragma once

#include <Arduino.h>

#include "config.h"

namespace input {

enum class Button : uint8_t {
  kNone = 0,
  kStart = 1 << 0,
  kReset = 1 << 1,
  kSkip = 1 << 2,
};

enum class ButtonEvent : uint8_t {
  kNone = 0,
  kPressed = 1 << 0,
  kReleased = 1 << 1,
  kLongPress = 1 << 2,
  kDoubleClick = 1 << 3,
};

void begin();
void poll();

uint8_t pressedMask();
uint8_t eventMask();
bool consume(uint8_t eventMask);

} input
