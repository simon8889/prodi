#pragma once

#include <Arduino.h>

namespace state {

enum class Phase : uint8_t {
  kFocus,
  kShortBreak,
  kLongBreak,
};

enum class TimerStatus : uint8_t {
  kIdle,
  kRunning,
  kPaused,
};

struct Snapshot {
  Phase phase = Phase::kFocus;
  TimerStatus status = TimerStatus::kIdle;
  uint16_t remainingSeconds = 0;
  uint16_t phaseDurationSeconds = 0;
  uint16_t cycleCount = 0;
  char activeTaskTitle[40] = {0};
  uint8_t taskCompleted = 0;
  uint8_t taskEstimated = 0;
  float progress = 0.0f;
  uint32_t updatedAtMs = 0;
};

const char* phaseLabel(Phase phase);
const char* phaseShortLabel(Phase phase);
const char* statusLabel(TimerStatus status);
float phaseProgress(const Snapshot& snapshot, uint32_t nowMs);

} state
