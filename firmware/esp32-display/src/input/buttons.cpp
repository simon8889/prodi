#include "input/buttons.h"

#include <Arduino.h>

#include <cstring>

namespace input {
namespace {

struct Channel {
  uint8_t pin = 0;
  Button button = Button::kNone;
  bool stable = true;
  bool lastRead = true;
  uint32_t changedAtMs = 0;
  uint32_t pressedAtMs = 0;
  uint8_t clicks = 0;
  uint32_t lastReleaseMs = 0;
  bool longFired = false;
};

Channel gChannels[3] = {
    {config::kStartButtonPin, Button::kStart, true, true, 0, 0, 0, 0, false},
    {config::kResetButtonPin, Button::kReset, true, true, 0, 0, 0, 0, false},
    {config::kSkipButtonPin, Button::kSkip, true, true, 0, 0, 0, 0, false},
};

uint8_t gPressed = 0;
uint8_t gEvents = 0;

}

void begin() {
  for (Channel& channel : gChannels) {
    pinMode(channel.pin, INPUT_PULLUP);
    channel.lastRead = digitalRead(channel.pin) == LOW;
    channel.stable = channel.lastRead;
    channel.changedAtMs = millis();
  }
  gPressed = 0;
  gEvents = 0;
}

void poll() {
  const uint32_t now = millis();

  for (Channel& channel : gChannels) {
    const bool reading = digitalRead(channel.pin) == LOW;

    if (reading != channel.lastRead) {
      channel.lastRead = reading;
      channel.changedAtMs = now;
      continue;
    }

    if (reading == channel.stable) {
      continue;
    }

    if (now - channel.changedAtMs < config::kButtonDebounceMs) {
      continue;
    }

    channel.stable = reading;

    if (reading) {
      channel.pressedAtMs = now;
      channel.longFired = false;
      gPressed |= static_cast<uint8_t>(channel.button);
      gEvents |= static_cast<uint8_t>(ButtonEvent::kPressed);
    } else {
      gPressed &= static_cast<uint8_t>(~static_cast<uint8_t>(channel.button));
      gEvents |= static_cast<uint8_t>(ButtonEvent::kReleased);

      const uint32_t heldMs = now - channel.pressedAtMs;
      const uint32_t sinceRelease = now - channel.lastReleaseMs;
      if (!channel.longFired && heldMs < config::kButtonLongPressMs &&
          sinceRelease < 320) {
        channel.clicks = static_cast<uint8_t>(channel.clicks + 1);
      } else {
        channel.clicks = 1;
      }
      channel.lastReleaseMs = now;

      if (channel.clicks >= 2) {
        channel.clicks = 0;
        gEvents |= static_cast<uint8_t>(ButtonEvent::kDoubleClick);
      }
    }
  }

  for (Channel& channel : gChannels) {
    if (!channel.stable || channel.longFired) {
      continue;
    }
    if (now - channel.pressedAtMs >= config::kButtonLongPressMs) {
      channel.longFired = true;
      channel.clicks = 0;
      gEvents |= static_cast<uint8_t>(ButtonEvent::kLongPress);
    }
  }
}

uint8_t pressedMask() {
  return gPressed;
}

uint8_t eventMask() {
  return gEvents;
}

bool consume(const uint8_t events) {
  const uint8_t pending = gEvents & events;
  gEvents &= static_cast<uint8_t>(~pending);
  return pending != 0;
}

} input
