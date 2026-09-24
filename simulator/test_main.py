"""Regression test for the simulator's stiction toggle: locks in that it
produces a signal the real classic detector actually distinguishes, not
just "some noise that looks different." See main.py's module docstring
for why the parameters here (period/amplitude/stick_band) aren't
arbitrary -- changing them without rechecking this test risks silently
breaking the pipeline's ground truth.
"""

import numpy as np
from main import Simulator
from valve_stiction_ml.classic import ellipse_stiction_index, kano_pattern_check


def zscore(x: np.ndarray) -> np.ndarray:
    std = x.std()
    return (x - x.mean()) / std if std > 0 else x - x.mean()


def collect_window(stiction_enabled: bool, settle: int = 50, window: int = 100):
    sim = Simulator(stiction_enabled=stiction_enabled, seed=0)
    for _ in range(settle):
        sim.step()
    pvs, ops = [], []
    for _ in range(window):
        pv, op = sim.step()
        pvs.append(pv)
        ops.append(op)
    return np.array(pvs), np.array(ops)


def test_stiction_enabled_is_detected_by_kano():
    pv, op = collect_window(stiction_enabled=True)
    assert kano_pattern_check(zscore(pv), zscore(op)) is True


def test_stiction_disabled_is_not_detected_by_kano():
    pv, op = collect_window(stiction_enabled=False)
    assert kano_pattern_check(zscore(pv), zscore(op)) is False


def test_stiction_enabled_has_higher_ellipse_index():
    pv_on, op_on = collect_window(stiction_enabled=True)
    pv_off, op_off = collect_window(stiction_enabled=False)

    idx_on = ellipse_stiction_index(zscore(pv_on), zscore(op_on))
    idx_off = ellipse_stiction_index(zscore(pv_off), zscore(op_off))

    assert idx_on > idx_off
