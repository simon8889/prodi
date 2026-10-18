#include "state/model.h"

namespace state {

const char* phaseLabel(const Phase phase) {
  switch (phase) {
    case Phase::kFocus:
      return "FOCUS";
    case Phase::kShortBreak:
      return "SHORT BREAK";
    case Phase::kLongBreak:
      return "LONG BREAK";
  }
  return "FOCUS";
}

const char* phaseShortLabel(const Phase phase) {
  switch (phase) {
    case Phase::kFocus:
      return "FOCUS";
    case Phase::kShortBreak:
      return "BREAK";
    case Phase::kLongBreak:
      return "LONG";
  }
  return "FOCUS";
}

const char* statusLabel(const TimerStatus status) {
  switch (status) {
    case TimerStatus::kIdle:
      return "READY";
    case TimerStatus::kRunning:
      return "RUNNING";
    case TimerStatus::kPaused:
      return "PAUSED";
  }
  return "READY";
}

float phaseProgress(const Snapshot& snapshot, const uint32_t nowMs) {
  if (snapshot.phaseDurationSeconds == 0) {
    return 0.0f;
  }
  if (snapshot.status != TimerStatus::kRunning) {
    return 1.0f - (static_cast<float>(snapshot.remainingSeconds) /
                   static_cast<float>(snapshot.phaseDurationSeconds));
  }
  const uint32_t elapsedMs = nowMs - snapshot.updatedAtMs;
  const float elapsedSeconds = static_cast<float>(elapsedMs) / 1000.0f;
  float remaining =
      static_cast<float>(snapshot.remainingSeconds) - elapsedSeconds;
  if (remaining < 0.0f) {
    remaining = 0.0f;
  }
  return 1.0f - (remaining / static_cast<float>(snapshot.phaseDurationSeconds));
}

} state
