//! Own Go commands.go f00311b; Beetle cdc.c source/version/license in parent.
use super::*;
pub(super) fn cmd_args(c: u8) -> Option<(usize, usize)> {
    Some(match c {
        1 | 4..=12 | 15..=17 | 19 | 21 | 22 | 26..=28 | 30 => (0, 0),
        2 => (3, 3),
        3 => (0, 1),
        13 | 29 => (2, 2),
        14 | 18 | 20 | 25 => (1, 1),
        _ => return None,
    })
}
impl Cdrom {
    fn ack(&mut self) {
        let s = self.status(false);
        self.result(s);
        self.write_irq(3)
    }
    pub(super) fn error(&mut self, code: u8) {
        let s = self.status(true);
        self.result(s);
        self.result(code);
        self.write_irq(5)
    }
    fn check_disc(&mut self) -> bool {
        if self.disc.is_none() || self.startup > 0 {
            self.error(0x80);
            false
        } else {
            true
        }
    }
    fn seek_time(&self, mut initial: i32, target: i32, motor: bool, paused: bool) -> i64 {
        let mut ret = 0;
        if !motor {
            initial = 0;
            ret += CPU_HZ
        }
        let d = (initial - target).abs();
        ret += ((d as i64 * CPU_HZ * 1000 / (72 * 60 * 75)) / 1000).max(20000);
        if d >= 2250 {
            ret += CPU_HZ * 300 / 1000
        } else if paused {
            ret += if self.mode & 128 != 0 {
                1237952
            } else {
                2475904
            }
        } else if (3..12).contains(&d) {
            ret += self.sector_time() * 4
        }
        ret + 12500
    }
    fn pre_seek(&mut self, target: i32) {
        self.cur_sector = target;
        self.seek_retry = 128;
        self.subq = self.disc.as_ref().unwrap().subq(target)
    }
    pub(super) fn command(&mut self, c: u8, a: &[u8]) -> i64 {
        match c {
            1 => self.ack(),
            2 => {
                if a[0] & 15 > 9
                    || a[0] > 0x99
                    || a[1] & 15 > 9
                    || a[1] > 0x59
                    || a[2] & 15 > 9
                    || a[2] > 0x74
                {
                    self.error(0x10);
                    return 0;
                }
                self.command_loc = unbcd(a[0]) * 4500 + unbcd(a[1]) * 75 + unbcd(a[2]) - 150;
                self.loc_dirty = true;
                self.ack()
            }
            3 => return self.play(a),
            4 | 5 => {
                if !self.check_disc() {
                    return 0;
                }
                self.ack();
                self.forward = c == 4;
                self.backward = c == 5
            }
            6 | 27 => {
                if self.check_disc() {
                    self.read_base()
                }
            }
            7 => {
                if !self.check_disc() {
                    return 0;
                }
                if self.drive != STOPPED {
                    self.error(0x20);
                    return 0;
                }
                self.ack();
                self.audio.clear();
                self.clear_aip();
                self.pipe_pos = 0;
                self.pipe_in = 0;
                self.sectors_read = 0;
                self.seek_target = self.cur_sector;
                self.hold_logical = false;
                self.drive = STANDBY;
                return CPU_HZ / 10;
            }
            8 => {
                if !self.check_disc() {
                    return 0;
                }
                self.ack();
                if self.drive == STOPPED {
                    return 5000;
                }
                self.audio.clear();
                self.clear_aip();
                self.pipe_pos = 0;
                self.pipe_in = 0;
                self.sectors_read = 0;
                self.drive = STOPPED;
                self.header_valid = false;
                return 33868;
            }
            9 => {
                if !self.check_disc() {
                    return 0;
                }
                self.ack();
                if self.drive == PAUSED || self.drive == STOPPED {
                    return 5000;
                }
                self.cur_sector -= self.sectors_read.min(4) as i32;
                self.sectors_read = 0;
                self.pipe_pos = 0;
                self.pipe_in = 0;
                self.clear_aip();
                self.seek_target = self.cur_sector;
                self.drive = PAUSED;
                self.psr = self.sector_time();
                let mul = if self.mode & 128 != 0 { 1 } else { 2 };
                return (1124584 + self.cur_sector as i64 * 42596 / 4500) * mul + 50000;
            }
            10 => {
                self.ack();
                self.muted = false;
                self.mode = 0x20;
                self.command_loc = 0;
                if self.disc.is_some() && self.startup <= 0 {
                    if [SEEKING, SEEKING_LOGICAL, SEEKING_LOGICAL2].contains(&self.drive)
                        && self.after_seek == PAUSED
                        && self.seek_target == 0
                    {
                        return 256;
                    }
                    self.audio.clear();
                    self.sb_in = 0;
                    self.pipe_pos = 0;
                    self.pipe_in = 0;
                    self.sectors_read = 0;
                    self.seek_target = 0;
                    self.psr = self
                        .seek_time(
                            self.cur_sector,
                            0,
                            self.drive != STOPPED,
                            self.drive == PAUSED,
                        )
                        .max(1625000);
                    self.pre_seek(0);
                    self.seek_finished = 0;
                    self.drive = if self.hold_logical {
                        SEEKING_LOGICAL
                    } else {
                        SEEKING
                    };
                    self.after_seek = PAUSED;
                    self.clear_aip();
                    return 4100000;
                }
                self.seek_finished = 1;
                return 70000;
            }
            11 => {
                self.muted = true;
                self.ack()
            }
            12 => {
                self.muted = false;
                self.ack()
            }
            13 => {
                self.filter_file = a[0];
                self.filter_chan = a[1];
                self.ack()
            }
            14 => {
                self.mode = a[0];
                self.ack()
            }
            15 => {
                let s = self.status(false);
                for v in [s, self.mode, 0, self.filter_file, self.filter_chan] {
                    self.result(v)
                }
                self.write_irq(3)
            }
            16 => {
                if !self.check_disc() {
                    return 0;
                }
                if !self.header_valid {
                    self.error(0x80);
                    return 0;
                }
                for i in 0..8 {
                    self.result(self.header[i])
                }
                self.write_irq(3)
            }
            17 => {
                if !self.check_disc() {
                    return 0;
                }
                for i in [1, 2, 3, 4, 5, 7, 8, 9] {
                    self.result(self.subq[i])
                }
                self.write_irq(3)
            }
            18 => {
                self.ack();
                return 44100 * 768 / 1000;
            }
            19 => {
                if !self.check_disc() {
                    return 0;
                }
                let d = self.disc.as_ref().unwrap();
                let first = bcd(d.tracks[0].number);
                let last = bcd(d.tracks.last().unwrap().number);
                let s = self.status(false);
                for v in [s, first, last] {
                    self.result(v)
                }
                self.write_irq(3)
            }
            20 => {
                if !self.check_disc() {
                    return 0;
                }
                let d = self.disc.as_ref().unwrap();
                let lba = if a[0] == 0 {
                    d.lead_out()
                } else {
                    let t = unbcd(a[0]);
                    let first = d.tracks[0].number;
                    let last = d.tracks.last().unwrap().number;
                    if a[0] & 15 > 9 || a[0] >> 4 > 9 || t < first || t > last {
                        self.error(0x10);
                        return 0;
                    }
                    d.tracks[(t - first) as usize].index1
                };
                let v = lba + 150;
                let s = self.status(false);
                for b in [s, bcd(v / 4500), bcd(v / 75 % 60)] {
                    self.result(b)
                }
                self.write_irq(3)
            }
            21 | 22 => {
                if !self.check_disc() {
                    return 0;
                }
                self.ack();
                self.seek_target = self.command_loc;
                if c == 21 {
                    self.sectors_read = 0;
                    self.pipe_pos = 0;
                    self.pipe_in = 0
                } else {
                    self.header_valid = false
                }
                self.psr = self.seek_time(
                    self.cur_sector,
                    self.seek_target,
                    self.drive != STOPPED,
                    self.drive == PAUSED,
                );
                self.pre_seek(self.seek_target);
                self.seek_finished = 0;
                self.drive = if c == 21 { SEEKING_LOGICAL } else { SEEKING };
                self.after_seek = STANDBY;
                self.clear_aip();
                return self.psr;
            }
            25 => self.test(a[0]),
            26 => {
                if !self.check_disc() {
                    return 0;
                }
                self.ack();
                return 33868;
            }
            28 => {}
            29 => self.ack(),
            30 => {
                self.header_valid = false;
                self.ack();
                if !self.check_disc() {
                    return 26000;
                }
                let t = 30000000
                    + self.seek_time(
                        self.cur_sector,
                        0,
                        self.drive != STOPPED,
                        self.drive == PAUSED,
                    );
                self.seek_target = 0;
                self.hold_logical = false;
                self.drive = PAUSED;
                self.clear_aip();
                return t;
            }
            _ => {}
        }
        0
    }
    pub(super) fn command_part2(&mut self, c: u8) -> i64 {
        match c {
            7 | 8 => {
                self.psr = 0;
                let s = self.status(false);
                self.result(s);
                self.write_irq(2)
            }
            9 | 18 | 30 => {
                let s = self.status(false);
                self.result(s);
                self.write_irq(2)
            }
            10 | 21 | 22 => {
                if ![SEEKING, SEEKING_LOGICAL, SEEKING_LOGICAL2].contains(&self.drive) {
                    if self.seek_finished != 0 {
                        self.begin_results();
                        let s = self.status(false);
                        if self.seek_finished < 0 {
                            self.result(s | 4);
                            self.result(4);
                            self.write_irq(5)
                        } else {
                            self.result(s);
                            self.write_irq(2)
                        }
                    }
                    return 0;
                }
                return self.psr.max(256);
            }
            26 => {
                let s = self.status(false);
                if self.is_psx {
                    for v in [s, 0, 0x20, 0] {
                        self.result(v)
                    }
                    for b in self.disc_id {
                        self.result(b)
                    }
                    self.write_irq(2)
                } else {
                    for v in [s | 8, 0x90, 0, 0, 255, 0, 0, 0] {
                        self.result(v)
                    }
                    self.write_irq(5)
                }
            }
            _ => {}
        }
        0
    }
    fn play(&mut self, a: &[u8]) -> i64 {
        if !self.check_disc() {
            return 0;
        }
        self.clear_aip();
        self.ack();
        self.forward = false;
        self.backward = false;
        if !a.is_empty() && a[0] != 0 {
            let d = self.disc.as_ref().unwrap();
            let first = d.tracks[0].number;
            let t = unbcd(a[0]).clamp(first, d.tracks.last().unwrap().number);
            self.seek_target = d.tracks[(t - first) as usize].index1;
            self.audio.clear();
            self.pipe_pos = 0;
            self.pipe_in = 0;
            self.sectors_read = 0;
            self.track_match = t;
            self.start_seek_play()
        } else if self.loc_dirty || self.drive != PLAYING {
            self.audio.clear();
            self.pipe_pos = 0;
            self.pipe_in = 0;
            self.sectors_read = 0;
            self.seek_target = if self.loc_dirty {
                self.command_loc
            } else {
                self.cur_sector
            };
            self.track_match = -1;
            self.start_seek_play()
        }
        self.loc_dirty = false;
        0
    }
    fn start_seek_play(&mut self) {
        self.psr = self.seek_time(
            self.cur_sector,
            self.seek_target,
            self.drive != STOPPED,
            self.drive == PAUSED,
        );
        self.header_valid = false;
        self.pre_seek(self.seek_target);
        self.report_last = 255;
        self.seek_finished = 0;
        self.drive = SEEKING;
        self.after_seek = PLAYING;
    }
    fn read_base(&mut self) {
        if !self.is_psx {
            self.error(0x40);
            return;
        }
        self.ack();
        if [SEEKING_LOGICAL, SEEKING_LOGICAL2].contains(&self.drive)
            && self.seek_target == self.command_loc
            && self.after_seek == READING
        {
            self.loc_dirty = false;
            return;
        }
        if self.loc_dirty || self.drive != READING {
            self.clear_aip();
            self.audio.clear();
            self.sb_in = 0;
            self.pipe_pos = 0;
            self.pipe_in = 0;
            self.sectors_read = 0;
            if self.loc_dirty {
                self.seek_target = self.command_loc
            } else if self.drive != PAUSED && self.drive != STANDBY {
                self.seek_target = self.cur_sector
            }
            self.psr = self.sector_time()
                + self.seek_time(
                    self.cur_sector,
                    self.seek_target,
                    self.drive != STOPPED,
                    self.drive == PAUSED,
                );
            self.header_valid = false;
            self.pre_seek(self.seek_target);
            self.seek_finished = 0;
            self.drive = SEEKING_LOGICAL;
            self.after_seek = READING
        }
        self.loc_dirty = false;
    }
    fn test(&mut self, sub: u8) {
        let data: &[u8] = match sub {
            0..=4 | 0x10..=0x1a => {
                self.ack();
                return;
            }
            5 => &[0, 0],
            0x20 => &[0x97, 1, 0x10, 0xc2],
            0x21 => &[1],
            0x22 => b"for U/C",
            0x23 | 0x24 => b"CXD2545Q",
            0x25 => b"CXD1815Q",
            _ => {
                self.error(0x10);
                return;
            }
        };
        for &v in data {
            self.result(v)
        }
        self.write_irq(3)
    }
}
