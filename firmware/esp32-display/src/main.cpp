#include <Arduino.h>

#include "config.h"
#include "display/oled.h"
#include "input/buttons.h"
#include "net/ws_client.h"

namespace {

enum class Screen : uint8_t {
  kSplash,
  kLink,
  kTimer,
};

Screen gScreen = Screen::kSplash;
uint32_t gSplashUntilMs = 0;
uint32_t gLastRenderMs = 0;
uint32_t gLastSocketSeenMs = 0;

void showTimerScreen(const uint32_t now) {
  const net::LinkState link = net::linkState();
  const bool stale =
      net::connected() && (now - net::lastMessageAtMs()) > config::kStaleAfterMs;

  if (link != net::LinkState::kOnline) {
    gScreen = Screen::kLink;
    const char* detail = "connecting";
    display::showConnecting(detail);
    return;
  }

  gScreen = Screen::kTimer;
  display::render(net::snapshot(), now, !stale, net::linkLabel());
}

void handleButtons() {
  input::poll();

  if (!net::connected()) {
    return;
  }

  if (!input::consume(static_cast<uint8_t>(input::ButtonEvent::kPressed))) {
    return;
  }

  const uint8_t pressed = input::pressedMask();

  if (pressed & static_cast<uint8_t>(input::Button::kStart)) {
    const state::Snapshot& current = net::snapshot();
    if (current.status == state::TimerStatus::kRunning) {
      net::sendCommand("timer.pause");
    } else {
      net::sendCommand("timer.start");
    }
  }

  if (pressed & static_cast<uint8_t>(input::Button::kReset)) {
    net::sendCommand("timer.reset");
  }

  if (pressed & static_cast<uint8_t>(input::Button::kSkip)) {
    net::sendCommand("timer.skip");
  }
}

}

void setup() {
  Serial.begin(115200);
  delay(40);

  pinMode(LED_BUILTIN, OUTPUT);
  digitalWrite(LED_BUILTIN, LOW);

  display::begin();
  input::begin();
  net::onConnectCallback([]() {
    digitalWrite(LED_BUILTIN, HIGH);
  });

  display::showSplash();
  gScreen = Screen::kSplash;
  gSplashUntilMs = millis() + 1200;

  net::begin();
}

void loop() {
  const uint32_t now = millis();

  if (gScreen == Screen::kSplash && now >= gSplashUntilMs) {
    gScreen = Screen::kLink;
  }

  handleButtons();

  if (now - gLastSocketSeenMs > 250) {
    gLastSocketSeenMs = now;
    net::loop();
  }

  if (now - gLastRenderMs < config::kDisplayRefreshMs) {
    return;
  }
  gLastRenderMs = now;

  if (gScreen == Screen::kSplash) {
    return;
  }

  showTimerScreen(now);

  if (!net::connected()) {
    digitalWrite(LED_BUILTIN, LOW);
  }
}
