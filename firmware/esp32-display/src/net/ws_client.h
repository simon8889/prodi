#pragma once

#include <Arduino.h>

#include "state/model.h"

namespace net {

enum class LinkState : uint8_t {
  kIdle,
  kConnectingWifi,
  kConnectingSocket,
  kOnline,
};

void begin();
void loop();

LinkState linkState();
bool connected();
const char* linkLabel();

const state::Snapshot& snapshot();
uint32_t lastMessageAtMs();

bool sendCommand(const char* type);
bool sendPing();

void onConnectCallback(void (*handler)());

} net
