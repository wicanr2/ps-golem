//! Original frontend key positions (012 / 026 section14).
//! Source: own frontend/ebiten/input.go at 6e66bd5, GPL-3.0-or-later.
use sdl2::{
    controller::{Axis, Button, GameController},
    keyboard::Scancode,
};
pub const KEYS: [(Scancode, u16); 14] = [
    (Scancode::Up, 16),
    (Scancode::Right, 32),
    (Scancode::Down, 64),
    (Scancode::Left, 128),
    (Scancode::Z, 16384),
    (Scancode::X, 8192),
    (Scancode::A, 32768),
    (Scancode::S, 4096),
    (Scancode::Q, 1024),
    (Scancode::W, 2048),
    (Scancode::Num1, 256),
    (Scancode::Num3, 512),
    (Scancode::Return, 8),
    (Scancode::Backspace, 1),
];
pub fn keyboard(pressed: impl Fn(Scancode) -> bool) -> u16 {
    KEYS.iter()
        .filter(|(key, _)| pressed(*key))
        .fold(0, |v, (_, b)| v | b)
}
pub fn axes(x: i16, y: i16, l2: i16, r2: i16) -> u16 {
    // Same 0.2 dead zone as 020. Widen before abs to handle i16::MIN.
    let mut v = 0;
    if (x as i32) < -6553 {
        v |= 128;
    }
    if x as i32 > 6553 {
        v |= 32;
    }
    if (y as i32) < -6553 {
        v |= 16;
    }
    if y as i32 > 6553 {
        v |= 64;
    }
    if l2 > 6553 {
        v |= 256;
    }
    if r2 > 6553 {
        v |= 512;
    }
    v
}
pub fn controller(p: &GameController) -> u16 {
    use Button::*;
    let map = [
        (DPadUp, 16),
        (DPadRight, 32),
        (DPadDown, 64),
        (DPadLeft, 128),
        (A, 16384),
        (B, 8192),
        (X, 32768),
        (Y, 4096),
        (LeftShoulder, 1024),
        (RightShoulder, 2048),
        (Start, 8),
        (Back, 1),
    ];
    map.iter()
        .filter(|(b, _)| p.button(*b))
        .fold(0, |v, (_, b)| v | b)
        | axes(
            p.axis(Axis::LeftX),
            p.axis(Axis::LeftY),
            p.axis(Axis::TriggerLeft),
            p.axis(Axis::TriggerRight),
        )
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn positions_and_chords() {
        for (key, bit) in KEYS {
            assert_eq!(keyboard(|k| k == key), bit);
        }
        assert_eq!(
            keyboard(|k| k == Scancode::Z || k == Scancode::Left),
            16384 | 128
        );
        assert_eq!(keyboard(|_| false), 0);
    }
    #[test]
    fn analog_edges() {
        assert_eq!(axes(-6553, 6553, 6553, 0), 0);
        assert_eq!(axes(i16::MIN, i16::MAX, 6554, 6554), 128 | 64 | 256 | 512);
        assert_eq!(axes(6554, -6554, 0, 0), 32 | 16);
    }
}
