#pragma once

#include <U8g2lib.h>

#include "config.h"
#include "state/model.h"

namespace display {

bool begin();

void showSplash();
void showConnecting(const char* detail);
void showMessage(const char* title, const char* detail);
void render(const state::Snapshot& snapshot, uint32_t nowMs, bool socketConnected,
            const char* linkLabel);
void powerOff();

} display
