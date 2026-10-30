//! Ordinary NO3 compatibility clock resources, spec026 §16.5.
//! Source: own apps/sotn/hd_call_clock.go at 1f142c7, GPL-3.0-or-later.
//! These are cache/read footprints, never guest opcode decoding/execution.
use super::call::Layer;
use psgolem_core::cpu::State;
const READS: &[u32] = &[
    0x80011f24, 0x80011f28, 0x80011f3c, 0x8001360c, 0x80013630, 0x80013634, 0x80013638, 0x80013904,
    0x800ecc50, 0x800ecc98, 0x800eccb4, 0x800eccbc, 0x800ecce0, 0x800eccf8, 0x800ecd2c, 0x800ecd4c,
    0x800ecd54, 0x800ecd6c, 0x800ecd80, 0x800ecd84, 0x800ecd9c, 0x800ecda4, 0x800ecdac, 0x800ecdcc,
    0x800ece40, 0x800ece78, 0x800ecebc, 0x800ecedc, 0x800ecee0, 0x800ecef4, 0x800ecf04, 0x800ecf14,
    0x800ecfac, 0x800ecfb0, 0x800ecfdc, 0x800ecff4, 0x800ed038, 0x800ed0b0, 0x800ed0ec, 0x800ed104,
    0x800ed120, 0x800ed128, 0x800ed134, 0x800ed13c, 0x800ed148, 0x800ed150, 0x800ed15c, 0x800ed168,
    0x800ed178, 0x800ed188, 0x800ed1a0, 0x800ed20c, 0x800ed258, 0x800ed2c8, 0x800ed2e8, 0x800ed2ec,
    0x800ed300, 0x800ed310, 0x800ed32c, 0x800ed3c4, 0x800ed3c8, 0x800ed3f0, 0x800ed40c, 0x800ed450,
    0x800ed4c8, 0x800ed4f8, 0x800ed4fc, 0x800ed500, 0x800ed504, 0x800ed508, 0x800ed50c, 0x800ed510,
    0x800ed514, 0x800ed518, 0x800ed51c,
];
pub(super) struct Clock {
    pub state: State,
    pub costs: Vec<u32>,
}
impl Clock {
    fn run(&mut self, first: u32, last: u32) {
        for pc in (first..=last).step_by(4) {
            let p = pc & 0x1fff_ffff;
            let w = (p >> 2) & 3;
            let line = &mut self.state.icache[((p >> 4) & 255) as usize];
            let mut cost = 1;
            if line.tag != p >> 12 || line.valid & (1 << w) == 0 {
                let fill = ((15 << w) & 15) as u8;
                line.valid = if line.tag == p >> 12 {
                    line.valid | fill
                } else {
                    fill
                };
                line.tag = p >> 12;
                cost += 7 - w;
            }
            if READS.contains(&pc) {
                cost += 4;
            }
            self.state.now += cost as u64;
            self.state.cost = cost as u64;
            self.costs.push(cost);
        }
    }
    fn product(&mut self) {
        self.state.muldiv_done = self.state.now + 7;
    }
    fn await_product(&mut self, pc: u32) {
        self.run(pc, pc);
        if self.state.now < self.state.muldiv_done {
            let wait = self.state.muldiv_done - self.state.now;
            self.state.now += wait;
            self.state.cost += wait;
            *self.costs.last_mut().unwrap() += wait as u32;
        }
    }
    fn mode(&mut self) {
        for (a, b) in [
            (0x800135f0, 0x8001361c),
            (0x80013900, 0x80013910),
            (0x80013930, 0x80013934),
            (0x8001393c, 0x80013940),
            (0x80013948, 0x8001394c),
            (0x80013620, 0x80013628),
            (0x80013b24, 0x80013b28),
            (0x80013b34, 0x80013ba4),
            (0x8001362c, 0x80013644),
        ] {
            self.run(a, b);
        }
    }
    pub(super) fn prepare(state: &State, layers: &[Layer; 2]) -> Result<Self, String> {
        if state.now > u64::MAX - 2_000_000 {
            return Err("時鐘溢位".into());
        }
        let mut c = Self {
            state: state.clone(),
            costs: Vec::with_capacity(60000),
        };
        c.run(0x800ecc10, 0x800ecd10);
        c.run(0x800ecd28, 0x800ecdf8);
        c.run(0x800ece14, 0x800ece38);
        let mut sprites = 0;
        for (row, ids) in layers[0].ids.chunks_exact(17).enumerate() {
            let _ = row;
            c.run(0x800ece3c, 0x800ece6c);
            c.product();
            c.run(0x800ece70, 0x800ece78);
            c.await_product(0x800ece7c);
            c.run(0x800ece80, 0x800ece8c);
            for &id in ids {
                c.run(0x800ece90, 0x800ecec8);
                if id != 0 {
                    c.run(0x800ececc, 0x800ecf0c);
                    c.run(0x800ecf14, 0x800ecf24);
                    if sprites < 640 {
                        c.run(0x800ecf28, 0x800ecf40);
                        c.run(0x800ecf50, 0x800ed004);
                        sprites += 1;
                    }
                }
                c.run(0x800ed008, 0x800ed018);
            }
            c.run(0x800ed01c, 0x800ed028);
        }
        c.run(0x800ed02c, 0x800ed034);
        for _ in 0..24 {
            c.run(0x800ed038, 0x800ed048);
            c.run(0x800ed04c, 0x800ed068);
            c.run(0x800ed070, 0x800ed088);
            c.mode();
            c.run(0x800ed08c, 0x800ed0a4);
            c.run(0x80011f18, 0x80011f50);
            c.run(0x800ed0a8, 0x800ed0c8);
        }
        c.run(0x800ed0cc, 0x800ed0cc);
        c.run(0x800ed0d0, 0x800ed0e8);
        for layer in 0..16 {
            c.run(0x800ed0ec, 0x800ed0f8);
            c.run(0x800ed104, 0x800ed114);
            if layer == 0 {
                c.run(0x800ed118, 0x800ed1b8);
                c.run(0x800ed1c4, 0x800ed1d4);
                c.run(0x800ed1ec, 0x800ed200);
                for ids in layers[1].ids.chunks_exact(17) {
                    c.run(0x800ed204, 0x800ed220);
                    c.run(0x800ed228, 0x800ed24c);
                    c.product();
                    c.run(0x800ed250, 0x800ed258);
                    c.await_product(0x800ed25c);
                    c.run(0x800ed260, 0x800ed26c);
                    for &id in ids {
                        c.run(0x800ed270, 0x800ed280);
                        c.run(0x800ed29c, 0x800ed2d4);
                        if id != 0 {
                            c.run(0x800ed2d8, 0x800ed318);
                            c.run(0x800ed32c, 0x800ed33c);
                            if sprites < 640 {
                                c.run(0x800ed340, 0x800ed358);
                                c.run(0x800ed368, 0x800ed41c);
                                sprites += 1;
                            }
                        }
                        c.run(0x800ed420, 0x800ed430);
                    }
                    c.run(0x800ed434, 0x800ed440);
                }
                c.run(0x800ed444, 0x800ed44c);
                for _ in 0..24 {
                    c.run(0x800ed450, 0x800ed460);
                    c.run(0x800ed464, 0x800ed480);
                    c.run(0x800ed488, 0x800ed4a0);
                    c.mode();
                    c.run(0x800ed4a4, 0x800ed4bc);
                    c.run(0x80011f18, 0x80011f50);
                    c.run(0x800ed4c0, 0x800ed4e0);
                }
            }
            c.run(0x800ed4e4, 0x800ed4f4);
        }
        c.run(0x800ed4f8, 0x800ed528);
        Ok(c)
    }
}
