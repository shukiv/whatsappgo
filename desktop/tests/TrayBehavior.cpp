#include "traybehavior.h"

#include <cstdlib>

int main()
{
    const bool minimizesBehindAvailableTray =
        TrayBehavior::shouldHideWindow(QWindow::Minimized, true);
    const bool neverHidesWithoutTray =
        !TrayBehavior::shouldHideWindow(QWindow::Minimized, false);
    const bool normalWindowStaysVisible =
        !TrayBehavior::shouldHideWindow(QWindow::Windowed, true);
    const bool lifecycleMatchesAvailability =
        TrayBehavior::shouldKeepRunning(true) && !TrayBehavior::shouldKeepRunning(false);
    return minimizesBehindAvailableTray && neverHidesWithoutTray
            && normalWindowStaysVisible && lifecycleMatchesAvailability
        ? EXIT_SUCCESS
        : EXIT_FAILURE;
}
