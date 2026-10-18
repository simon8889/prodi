#include "display/oled.h"

#include <U8g2lib.h>

#include <cmath>
#include <cstdio>
#include <cstring>

namespace display {
namespace {

U8G2_SSD1306_128X64_NONAME_F_HW_I2C gDisplay(U8G2_R0, U8X8_PIN_NONE);

constexpr int16_t kWidth = config::kDisplayWidth;
constexpr int16_t kHeight = config::kDisplayHeight;
constexpr int16_t kBarHeight = 5;
constexpr int16_t kBarTop = 45;

void drawStatusBar(const state::Snapshot& snapshot, const bool socketConnected,
                   const char* linkLabel) {
  gDisplay.setFont(u8g2_font_6x10_tf);
  gDisplay.drawUTF8(2, 8, state::phaseShortLabel(snapshot.phase));

  char right[16];
  snprintf(right, sizeof(right), "%s", linkLabel != nullptr ? linkLabel : "IDLE");
  const int16_t rightWidth = gDisplay.getUTF8Width(right);
  gDisplay.drawUTF8(kWidth - rightWidth - 2, 8, right);

  if (socketConnected) {
    gDisplay.drawDisc(kWidth - rightWidth - 10, 6, 2);
  } else {
    gDisplay.drawCircle(kWidth - rightWidth - 10, 6, 2);
  }

  gDisplay.drawHLine(0, 11, kWidth);
}

int16_t remainingSeconds(const state::Snapshot& snapshot, const uint32_t nowMs) {
  int32_t seconds = snapshot.remainingSeconds;
  if (snapshot.status == state::TimerStatus::kRunning) {
    const int32_t elapsed =
        static_cast<int32_t>((nowMs - snapshot.updatedAtMs) / 1000U);
    seconds -= elapsed;
  }
  if (seconds < 0) {
    seconds = 0;
  }
  return static_cast<int16_t>(seconds);
}

void drawTimer(const state::Snapshot& snapshot, const uint32_t nowMs) {
  const int16_t seconds = remainingSeconds(snapshot, nowMs);

  char buffer[8];
  snprintf(buffer, sizeof(buffer), "%02d:%02d", seconds / 60, seconds % 60);

  gDisplay.setFont(u8g2_font_logisoso30_tr);
  const int16_t textWidth = gDisplay.getUTF8Width(buffer);
  gDisplay.drawUTF8((kWidth - textWidth) / 2, 41, buffer);
}

void drawProgressBar(const float progress) {
  float clamped = progress;
  if (clamped < 0.0f) {
    clamped = 0.0f;
  }
  if (clamped > 1.0f) {
    clamped = 1.0f;
  }

  gDisplay.drawFrame(0, kBarTop, kWidth, kBarHeight + 2);
  const int16_t innerWidth = kWidth - 4;
  const int16_t filled = static_cast<int16_t>(innerWidth * clamped);
  if (filled > 0) {
    gDisplay.drawBox(2, kBarTop + 2, filled, kBarHeight - 2);
  }
}

void drawTaskLine(const state::Snapshot& snapshot) {
  gDisplay.setFont(u8g2_font_5x8_tf);

  char prefix[10];
  if (snapshot.taskEstimated > 0) {
    snprintf(prefix, sizeof(prefix), "%u/%u ",
             static_cast<unsigned>(snapshot.taskCompleted),
             static_cast<unsigned>(snapshot.taskEstimated));
  } else {
    snprintf(prefix, sizeof(prefix), "");
  }

  const int16_t prefixWidth = gDisplay.getUTF8Width(prefix);
  gDisplay.drawUTF8(2, 61, prefix);

  const int16_t available = kWidth - prefixWidth - 4;
  if (available <= 12) {
    return;
  }

  if (snapshot.activeTaskTitle[0] == '\0') {
    if (snapshot.phase == state::Phase::kFocus) {
      gDisplay.drawUTF8(prefixWidth + 2, 61, "No active task");
    } else {
      gDisplay.drawUTF8(prefixWidth + 2, 61, "Break time");
    }
    return;
  }

  char title[config::kTaskTitleLength];
  strncpy(title, snapshot.activeTaskTitle, sizeof(title) - 1);
  title[sizeof(title) - 1] = '\0';

  char* start = title;
  while (*start == ' ') {
    ++start;
  }

  while (gDisplay.getUTF8Width(start) > available) {
    size_t length = strlen(start);
    if (length <= 1) {
      break;
    }
    start[length - 2] = '\0';
    start[length - 1] = '\0';
    if (strlen(start) < 2) {
      break;
    }
  }

  char clipped[config::kTaskTitleLength];
  strncpy(clipped, start, sizeof(clipped) - 1);
  clipped[sizeof(clipped) - 1] = '\0';

  const int16_t room = available - 8;
  if (gDisplay.getUTF8Width(clipped) > room) {
    clipped[strlen(clipped) - 1] = '\0';
  }

  gDisplay.drawUTF8(prefixWidth + 2, 61, clipped);
}

}

bool begin() {
  gDisplay.begin(config::kSdaPin, config::kSclPin);
  gDisplay.setI2CAddress(config::kDisplayAddress * 2);
  gDisplay.clearBuffer();
  gDisplay.sendBuffer();
  return true;
}

void showSplash() {
  gDisplay.setFont(u8g2_font_logisoso20_tr);
  const char* name = "prodi";
  const int16_t width = gDisplay.getUTF8Width(name);
  gDisplay.clearBuffer();
  gDisplay.drawUTF8((kWidth - width) / 2, 30, name);
  gDisplay.setFont(u8g2_font_5x8_tf);
  const char* sub = "focus display";
  const int16_t subWidth = gDisplay.getUTF8Width(sub);
  gDisplay.drawUTF8((kWidth - subWidth) / 2, 44, sub);
  gDisplay.drawHLine(24, 48, 80);
  gDisplay.sendBuffer();
}

void showConnecting(const char* detail) {
  gDisplay.clearBuffer();
  gDisplay.setFont(u8g2_font_logisoso20_tr);
  const char* title = "prodi";
  const int16_t width = gDisplay.getUTF8Width(title);
  gDisplay.drawUTF8((kWidth - width) / 2, 22, title);

  gDisplay.setFont(u8g2_font_5x8_tf);
  const char* message = detail != nullptr ? detail : "connecting";
  const int16_t messageWidth = gDisplay.getUTF8Width(message);
  gDisplay.drawUTF8((kWidth - messageWidth) / 2, 36, message);

  gDisplay.drawFrame(0, 45, kWidth, kBarHeight + 2);
  for (int16_t x = 2; x < 30; x += 2) {
    gDisplay.drawBox(x, 47, 1, kBarHeight - 2);
  }
  gDisplay.sendBuffer();
}

void showMessage(const char* title, const char* detail) {
  gDisplay.clearBuffer();
  gDisplay.setFont(u8g2_font_logisoso20_tr);
  const char* headline = title != nullptr ? title : "prodi";
  const int16_t width = gDisplay.getUTF8Width(headline);
  gDisplay.drawUTF8((kWidth - width) / 2, 26, headline);

  if (detail != nullptr && detail[0] != '\0') {
    gDisplay.setFont(u8g2_font_5x8_tf);
    const int16_t detailWidth = gDisplay.getUTF8Width(detail);
    gDisplay.drawUTF8((kWidth - detailWidth) / 2, 40, detail);
  }
  gDisplay.sendBuffer();
}

void render(const state::Snapshot& snapshot, const uint32_t nowMs,
            const bool socketConnected, const char* linkLabel) {
  gDisplay.clearBuffer();
  drawStatusBar(snapshot, socketConnected, linkLabel);
  drawTimer(snapshot, nowMs);
  drawProgressBar(state::phaseProgress(snapshot, nowMs));
  drawTaskLine(snapshot);
  gDisplay.sendBuffer();
}

void powerOff() {
  gDisplay.clear();
  gDisplay.clearBuffer();
  gDisplay.sendBuffer();
}

} display
