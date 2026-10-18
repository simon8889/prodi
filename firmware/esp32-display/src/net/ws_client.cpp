#include "net/ws_client.h"

#include <Arduino.h>
#include <ArduinoJson.h>
#include <WebSocketsClient.h>
#include <WiFi.h>

#include <cstdio>
#include <cstring>

#include "config.h"
#include "secrets.h"

namespace net {
namespace {

WebSocketsClient gSocket;
state::Snapshot gSnapshot;

LinkState gLinkState = LinkState::kIdle;
bool gSocketConnected = false;
bool gWifiConnected = false;
uint32_t gLastMessageAtMs = 0;
uint32_t gLastWifiAttemptMs = 0;
uint32_t gLastPingMs = 0;
void (*gOnConnect)() = nullptr;

char gSocketUrl[160];

state::Phase parsePhase(const char* value) {
  if (value != nullptr && strcmp(value, "shortBreak") == 0) {
    return state::Phase::kShortBreak;
  }
  if (value != nullptr && strcmp(value, "longBreak") == 0) {
    return state::Phase::kLongBreak;
  }
  return state::Phase::kFocus;
}

state::TimerStatus parseStatus(const char* value) {
  if (value != nullptr && strcmp(value, "running") == 0) {
    return state::TimerStatus::kRunning;
  }
  if (value != nullptr && strcmp(value, "paused") == 0) {
    return state::TimerStatus::kPaused;
  }
  return state::TimerStatus::kIdle;
}

void copyText(char* destination, const size_t capacity, const char* source) {
  if (source == nullptr) {
    destination[0] = '\0';
    return;
  }
  strncpy(destination, source, capacity - 1);
  destination[capacity - 1] = '\0';
}

void buildSocketUrl() {
  if (WS_TOKEN[0] == '\0') {
    snprintf(gSocketUrl, sizeof(gSocketUrl), "%s", WS_PATH);
    return;
  }
  const bool hasQuery = strchr(WS_PATH, '?') != nullptr;
  snprintf(gSocketUrl, sizeof(gSocketUrl), "%s%stoken=%s", WS_PATH,
           hasQuery ? "&" : "?", WS_TOKEN);
}

void applyTick(JsonObjectConst payload) {
  if (payload.isNull()) {
    return;
  }

  gSnapshot.phase = parsePhase(payload["phase"] | "focus");
  gSnapshot.status = parseStatus(payload["status"] | "idle");
  gSnapshot.remainingSeconds = payload["remainingSeconds"] | 0;
  gSnapshot.phaseDurationSeconds = payload["phaseDurationSeconds"] | 0;
  gSnapshot.cycleCount = payload["cycleCount"] | 0;
  gSnapshot.taskCompleted = payload["taskCompletedPomodoros"] | 0;
  gSnapshot.taskEstimated = payload["taskEstimatedPomodoros"] | 0;
  gSnapshot.progress = payload["progress"] | 0.0f;
  copyText(gSnapshot.activeTaskTitle, sizeof(gSnapshot.activeTaskTitle),
           payload["activeTaskTitle"] | "");
  gSnapshot.updatedAtMs = millis();
  gLastMessageAtMs = gSnapshot.updatedAtMs;
}

void applySnapshot(JsonObjectConst payload) {
  if (payload.isNull()) {
    return;
  }

  JsonObjectConst timer = payload["timer"];
  if (!timer.isNull()) {
    gSnapshot.phase = parsePhase(timer["phase"] | "focus");
    gSnapshot.status = parseStatus(timer["status"] | "idle");
    gSnapshot.remainingSeconds = timer["remainingSeconds"] | 0;
    gSnapshot.phaseDurationSeconds = timer["phaseDurationSeconds"] | 0;
    gSnapshot.cycleCount = timer["cycleCount"] | 0;
  }

  JsonObjectConst active = payload["activeTask"];
  if (active.isNull()) {
    gSnapshot.activeTaskTitle[0] = '\0';
    gSnapshot.taskCompleted = 0;
    gSnapshot.taskEstimated = 0;
  } else {
    copyText(gSnapshot.activeTaskTitle, sizeof(gSnapshot.activeTaskTitle),
             active["title"] | "");
    gSnapshot.taskCompleted = active["completedPomodoros"] | 0;
    gSnapshot.taskEstimated = active["estimatedPomodoros"] | 0;
  }

  gSnapshot.updatedAtMs = millis();
  gLastMessageAtMs = gSnapshot.updatedAtMs;
}

void handleMessage(uint8_t* payload, const size_t length) {
  StaticJsonDocument<1536> document;
  const DeserializationError error = deserializeJson(document, payload, length);

  if (error) {
    return;
  }

  JsonObjectConst root = document.as<JsonObjectConst>();
  const char* type = root["type"] | "";

  if (strcmp(type, "tick") == 0) {
    applyTick(root["payload"]);
    return;
  }

  if (strcmp(type, "snapshot") == 0) {
    applySnapshot(root["payload"]);
    return;
  }

  if (strcmp(type, "error") == 0) {
    JsonObjectConst body = root["payload"];
    copyText(gSnapshot.activeTaskTitle, sizeof(gSnapshot.activeTaskTitle),
             body["message"] | "");
    gLastMessageAtMs = millis();
    return;
  }

  gLastMessageAtMs = millis();
}

void handleEvent(const WStype_t type, uint8_t* payload, const size_t length) {
  switch (type) {
    case WStype_CONNECTED:
      gSocketConnected = true;
      gLinkState = LinkState::kOnline;
      gLastPingMs = millis();
      gLastMessageAtMs = millis();
      if (gOnConnect != nullptr) {
        gOnConnect();
      }
      break;

    case WStype_DISCONNECTED:
    case WStype_CLOSED:
      gSocketConnected = false;
      gLinkState = LinkState::kConnectingSocket;
      break;

    case WStype_ERROR:
      gSocketConnected = false;
      gLinkState = LinkState::kConnectingSocket;
      break;

    case WStype_TEXT:
      handleMessage(payload, length);
      break;

    default:
      break;
  }
}

void startWifi() {
  gLinkState = LinkState::kConnectingWifi;
  WiFi.persistent(false);
  WiFi.mode(WIFI_STA);
  WiFi.setSleep(false);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
}

}

void begin() {
  gSnapshot.phase = state::Phase::kFocus;
  gSnapshot.status = state::TimerStatus::kIdle;
  gSnapshot.phaseDurationSeconds = 1500;
  gSnapshot.remainingSeconds = 1500;
  gSnapshot.updatedAtMs = millis();

  buildSocketUrl();
  gSocket.setReconnectInterval(config::kSocketReconnectMs);
  gSocket.setConnectionTimeout(8000);
  gSocket.setSendTimeout(8000);
  startWifi();
}

void loop() {
  const uint32_t now = millis();

  if (WiFi.status() != WL_CONNECTED) {
    gWifiConnected = false;
    gSocketConnected = false;
    gLinkState = LinkState::kConnectingWifi;
    if (now - gLastWifiAttemptMs > config::kWifiRetryMs) {
      gLastWifiAttemptMs = now;
      startWifi();
    }
    return;
  }

  if (!gWifiConnected) {
    gWifiConnected = true;
    gSocket.begin(WS_HOST, WS_PORT, gSocketUrl);
    gSocket.onEvent(handleEvent);
  }

  if (gSocketConnected) {
    gSocket.loop();
    if (now - gLastPingMs > config::kPingIntervalMs) {
      gLastPingMs = now;
      gSocket.sendTXT("{\"type\":\"ping\"}");
    }
    return;
  }

  gLinkState = LinkState::kConnectingSocket;
  gSocket.loop();
}

LinkState linkState() {
  return gLinkState;
}

bool connected() {
  return gSocketConnected;
}

const char* linkLabel() {
  switch (gLinkState) {
    case LinkState::kOnline:
      return "LINK";
    case LinkState::kConnectingWifi:
      return "WIFI";
    case LinkState::kConnectingSocket:
      return "SOCK";
    case LinkState::kIdle:
      break;
  }
  return "IDLE";
}

const state::Snapshot& snapshot() {
  return gSnapshot;
}

uint32_t lastMessageAtMs() {
  return gLastMessageAtMs;
}

bool sendCommand(const char* type) {
  if (!gSocketConnected || type == nullptr) {
    return false;
  }
  char buffer[64];
  snprintf(buffer, sizeof(buffer), "{\"type\":\"%s\"}", type);
  return gSocket.sendTXT(buffer);
}

bool sendPing() {
  if (!gSocketConnected) {
    return false;
  }
  return gSocket.sendTXT("{\"type\":\"ping\"}");
}

void onConnectCallback(void (*handler)()) {
  gOnConnect = handler;
}

} net
