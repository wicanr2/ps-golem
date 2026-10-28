//! CD controller, 009 / 013 §2 / 026 §9. Own Go f00311b port.
//! Original reference: beetle-psx-libretro e57860d82faa953dc0fc10f201fd583739bc2f7d,
//! mednafen/psx/cdc.c, GPL-2.0-or-later; this crate GPL-3.0-or-later.
use crate::disc::{bcd, unbcd, Disc, SECTOR_SIZE};
use std::sync::Arc;
mod audio;
mod commands;
#[cfg(test)]
mod tests;
use audio::Audio;
const CPU_HZ: i64 = 33868800;
const STANDBY: i32 = -2;
const PAUSED: i32 = -1;
const STOPPED: i32 = 0;
// Go iota starts at the declaration index, including the three explicit values.
const SEEKING: i32 = 3;
const SEEKING_LOGICAL: i32 = 4;
const SEEKING_LOGICAL2: i32 = 5;
const PLAYING: i32 = 6;
const READING: i32 = 7;
const RESETTING: i32 = 8;
#[derive(Clone)]
pub struct Cdrom {
    disc: Option<Arc<Disc>>,
    is_psx: bool,
    disc_id: [u8; 4],
    drive: i32,
    disc_changed: bool,
    startup: i64,
    selector: u8,
    args: [u8; 16],
    args_wp: u8,
    args_rp: u8,
    args_latch: u8,
    args_recv: [u8; 32],
    args_in: usize,
    results: [u8; 16],
    results_in: u8,
    results_wp: u8,
    results_rp: u8,
    dma: Vec<u8>,
    dma_pos: usize,
    sb: [u8; 2340],
    sb_in: usize,
    pipe: [[u8; SECTOR_SIZE]; 2],
    pipe_q: [[u8; 12]; 2],
    pipe_pos: usize,
    pipe_in: i32,
    subq: [u8; 12],
    header: [u8; 12],
    header_valid: bool,
    irq_buffer: u8,
    irq_mask: u8,
    ready: i64,
    filter_file: u8,
    filter_chan: u8,
    pending_cmd: u8,
    pending_phase: i32,
    pending: i64,
    mode: u8,
    after_seek: i32,
    forward: bool,
    backward: bool,
    muted: bool,
    track_match: i32,
    psr: i64,
    cur_sector: i32,
    sectors_read: u32,
    aip_irq: u8,
    aip_res: [u8; 16],
    aip_count: usize,
    seek_target: i32,
    seek_retry: u32,
    seek_finished: i32,
    hold_logical: bool,
    report_startup: i64,
    command_loc: i32,
    loc_dirty: bool,
    report_last: u8,
    audio: Audio,
    spu_counter: i64,
}
impl Default for Cdrom {
    fn default() -> Self {
        Self::new()
    }
}
impl Cdrom {
    pub fn new() -> Self {
        let mut c = Self {
            disc: None,
            is_psx: false,
            disc_id: [0; 4],
            drive: 0,
            disc_changed: false,
            startup: 0,
            selector: 0,
            args: [0; 16],
            args_wp: 0,
            args_rp: 0,
            args_latch: 0,
            args_recv: [0; 32],
            args_in: 0,
            results: [0; 16],
            results_in: 0,
            results_wp: 0,
            results_rp: 0,
            dma: Vec::new(),
            dma_pos: 0,
            sb: [0; 2340],
            sb_in: 0,
            pipe: [[0; SECTOR_SIZE]; 2],
            pipe_q: [[0; 12]; 2],
            pipe_pos: 0,
            pipe_in: 0,
            subq: [0; 12],
            header: [0; 12],
            header_valid: false,
            irq_buffer: 0,
            irq_mask: 0,
            ready: 0,
            filter_file: 0,
            filter_chan: 0,
            pending_cmd: 0,
            pending_phase: 0,
            pending: 0,
            mode: 0,
            after_seek: 0,
            forward: false,
            backward: false,
            muted: false,
            track_match: 0,
            psr: 0,
            cur_sector: 0,
            sectors_read: 0,
            aip_irq: 0,
            aip_res: [0; 16],
            aip_count: 0,
            seek_target: 0,
            seek_retry: 0,
            seek_finished: 0,
            hold_logical: false,
            report_startup: 0,
            command_loc: 0,
            loc_dirty: false,
            report_last: 0,
            audio: Audio::default(),
            spu_counter: 768,
        };
        c.soft_reset();
        c
    }
    pub fn insert(&mut self, d: Option<Arc<Disc>>) {
        self.disc = d;
        self.is_psx = false;
        self.disc_id = [0; 4];
        let Some(d) = &self.disc else {
            self.drive = STOPPED;
            return;
        };
        self.header_valid = false;
        self.startup = CPU_HZ;
        self.disc_changed = true;
        if !d.region().is_empty() {
            self.disc_id.copy_from_slice(d.region().as_bytes());
            self.is_psx = true
        }
    }
    fn soft_reset(&mut self) {
        self.audio.clear();
        self.audio.reset_volume();
        self.selector = 0;
        self.args = [0; 16];
        self.args_wp = 0;
        self.args_rp = 0;
        self.results = [0; 16];
        self.results_wp = 0;
        self.results_rp = 0;
        self.results_in = 0;
        self.ready = 0;
        self.irq_buffer = 0;
        self.irq_mask = 0;
        self.dma.clear();
        self.dma_pos = 0;
        self.sb_in = 0;
        self.pipe_pos = 0;
        self.pipe_in = 0;
        self.sectors_read = 0;
        self.subq = [0; 12];
        self.header = [0; 12];
        self.filter_file = 0;
        self.filter_chan = 0;
        self.pending_cmd = 0;
        self.pending_phase = 0;
        self.pending = 0;
        self.mode = 0x20;
        self.header_valid = false;
        self.drive = STOPPED;
        self.clear_aip();
        self.after_seek = STOPPED;
        self.seek_retry = 0;
        self.forward = false;
        self.backward = false;
        self.muted = false;
        self.track_match = 0;
        self.psr = 0;
        self.cur_sector = 0;
        self.report_last = 255;
        self.report_startup = 0;
        self.seek_finished = -1;
        self.seek_target = 0;
        self.command_loc = 0;
        self.loc_dirty = true;
        self.disc_changed = true;
    }
    pub fn irq(&self) -> bool {
        self.irq_buffer & (self.irq_mask & 31) != 0
    }
    fn clear_aip(&mut self) {
        self.aip_count = 0;
        self.aip_irq = 0
    }
    fn write_irq(&mut self, v: u8) {
        self.ready = 2000;
        self.irq_buffer = self.irq_buffer & 16 | v;
    }
    fn begin_results(&mut self) {
        self.results_in = 0;
        self.results_wp = 0;
        self.results_rp = 0;
        self.results = [0; 16]
    }
    fn result(&mut self, v: u8) {
        self.results[self.results_wp as usize] = v;
        self.results_wp = (self.results_wp + 1) & 15;
        self.results_in = (self.results_in + 1) & 31
    }
    fn status(&mut self, error: bool) -> u8 {
        let mut r = match self.drive {
            PLAYING => 128,
            READING => {
                if self.header_valid {
                    32
                } else {
                    64
                }
            }
            SEEKING | SEEKING_LOGICAL | SEEKING_LOGICAL2 => 64,
            _ => 0,
        };
        if self.disc.is_none() || self.disc_changed {
            r |= 16
        }
        if self.drive != STOPPED {
            r |= 2
        }
        if error {
            r |= 1
        }
        self.disc_changed = false;
        r
    }
    fn check_aip(&mut self) {
        if self.aip_irq != 0 && self.ready <= 0 {
            self.begin_results();
            for i in 0..self.aip_count {
                self.result(self.aip_res[i])
            }
            self.write_irq(self.aip_irq);
            self.clear_aip();
        }
    }
    fn set_aip(&mut self, irq: u8, r: &[u8]) {
        self.clear_aip();
        self.aip_count = r.len().min(16);
        self.aip_res[..self.aip_count].copy_from_slice(&r[..self.aip_count]);
        self.aip_irq = irq;
        self.check_aip();
    }
    pub fn read(&mut self, a: u32) -> u8 {
        match a & 3 {
            0 => {
                let mut r = self.selector & 3;
                if self.args_wp == self.args_rp {
                    r |= 8
                }
                if self.args_wp.wrapping_sub(self.args_rp) & 16 == 0 {
                    r |= 16
                }
                if self.results_in != 0 {
                    r |= 32
                }
                if self.dma_pos < self.dma.len() {
                    r |= 64
                }
                if self.pending > 0 && self.pending_phase <= 1 {
                    r |= 128
                }
                r
            }
            1 => {
                let r = self.results[self.results_rp as usize];
                self.results_rp = (self.results_rp + 1) & 15;
                self.results_in = self.results_in.wrapping_sub(1) & 31;
                r
            }
            2 => {
                if self.dma_pos < self.dma.len() {
                    let r = self.dma[self.dma_pos];
                    self.dma_pos += 1;
                    r
                } else {
                    0
                }
            }
            _ => {
                if self.selector & 1 != 0 {
                    0xe0 | self.irq_buffer
                } else {
                    255
                }
            }
        }
    }
    pub fn write(&mut self, a: u32, v: u8) {
        let a = a & 3;
        if a == 0 {
            self.selector = v & 3;
            return;
        }
        match (self.selector & 3) as u32 * 3 + a - 1 {
            0 => {
                self.pending = 13815;
                self.pending_cmd = v;
                self.pending_phase = -1;
                self.args_in = 0
            }
            1 => {
                self.args[(self.args_wp & 15) as usize] = v;
                self.args_wp = (self.args_wp + 1) & 31
            }
            2 => {
                if v & 128 != 0 {
                    if self.dma_pos >= self.dma.len() {
                        self.dma.clear();
                        if self.sb_in == 0 {
                            self.dma.extend_from_slice(&self.sb);
                            self.dma.resize(4096, 0)
                        } else {
                            self.dma.extend_from_slice(&self.sb[..self.sb_in]);
                            self.sb_in = 0
                        }
                        self.dma_pos = 0
                    }
                } else if v & 64 != 0 {
                    self.dma_pos = (self.dma_pos + 4).min(self.dma.len())
                } else {
                    self.dma.clear();
                    self.dma_pos = 0
                }
                if v & 32 != 0 {
                    self.irq_buffer |= 16
                }
            }
            4 => self.irq_mask = v,
            7 => self.audio.pending_vol[0][0] = v,
            8 => self.audio.pending_vol[0][1] = v,
            9 => self.audio.pending_vol[1][1] = v,
            10 => self.audio.pending_vol[1][0] = v,
            11 => {
                if v & 32 != 0 {
                    self.audio.vol = self.audio.pending_vol
                }
            }
            5 => {
                self.irq_buffer &= !v;
                if v & 128 != 0 {
                    self.soft_reset()
                }
                if v & 64 != 0 {
                    self.args_wp = 0;
                    self.args_rp = 0
                }
            }
            _ => {}
        }
    }
    pub fn dma_read(&mut self) -> u32 {
        let mut v = 0;
        for i in 0..4 {
            if self.dma_pos < self.dma.len() {
                v |= (self.dma[self.dma_pos] as u32) << (8 * i);
                self.dma_pos += 1
            }
        }
        v
    }
    /// Go Tick(clocks, nil), retained for standalone controller tests.
    pub fn tick(&mut self, clocks: i64) {
        self.tick_inner(clocks, None);
    }
    pub fn tick_with_spu(&mut self, clocks: i64, spu: &mut crate::spu::Spu) {
        self.tick_inner(clocks, Some(spu));
    }
    pub fn spu_counter(&self) -> i64 {
        self.spu_counter
    }
    #[cfg(test)]
    pub(crate) fn audio_clock_state(&self) -> [u64; 5] {
        [
            self.audio.size as u64,
            self.audio.freq as u64,
            self.audio.read as u64,
            self.audio.pos as u64,
            self.audio.phase as u64,
        ]
    }
    fn tick_inner(&mut self, mut clocks: i64, mut spu: Option<&mut crate::spu::Spu>) {
        while clocks > 0 {
            let mut chunk = clocks;
            if spu.is_some() && self.spu_counter > 0 {
                chunk = chunk.min(self.spu_counter);
            }
            for v in [self.psr, self.pending] {
                if v > 0 {
                    chunk = chunk.min(v)
                }
            }
            if self.startup > 0 {
                chunk = chunk.min(self.startup);
                self.startup -= chunk;
                if self.startup <= 0 {
                    self.seek_target = self.cur_sector;
                    self.hold_logical = false;
                    self.drive = PAUSED
                }
            }
            if self.irq_buffer & 15 == 0 {
                if self.ready > 0 {
                    chunk = chunk.min(self.ready);
                    self.ready -= chunk
                }
            }
            self.check_aip();
            if self.psr > 0 {
                self.psr -= chunk;
                if self.report_startup > 0 {
                    self.report_startup -= chunk
                }
                if self.psr <= 0 {
                    match self.drive {
                        RESETTING => {
                            let s = self.status(false);
                            self.set_aip(2, &[s]);
                            self.muted = false;
                            self.audio.clear();
                            self.sb_in = 0;
                            self.pipe_pos = 0;
                            self.pipe_in = 0;
                            self.sectors_read = 0;
                            self.mode = 0x20;
                            self.cur_sector = 0;
                            self.command_loc = 0;
                            self.seek_target = 0;
                            self.hold_logical = false;
                            self.seek_finished = 1;
                            self.drive = PAUSED;
                            self.clear_aip()
                        }
                        SEEKING => {
                            self.cur_sector = self.seek_target;
                            self.hold_logical = false;
                            self.drive = self.after_seek;
                            self.seek_finished = 1;
                            self.report_startup = 24000000;
                            if self.drive == PAUSED || self.drive == STANDBY {
                                self.cur_sector = (self.cur_sector - 9).max(-150)
                            }
                            self.psr = self.sector_time()
                        }
                        SEEKING_LOGICAL => {
                            self.cur_sector = self.seek_target;
                            self.hold_logical = true;
                            self.drive = SEEKING_LOGICAL2
                        }
                        _ => {}
                    }
                    if [PAUSED, STANDBY, READING, PLAYING, SEEKING_LOGICAL2].contains(&self.drive) {
                        self.play_read()
                    }
                }
            }
            if self.pending > 0 {
                self.pending -= chunk;
                if self.pending <= 0 && self.ready > 0 {
                    self.pending = self.ready
                } else if self.pending <= 0 {
                    self.run_pending()
                }
            }
            if let Some(s) = spu.as_deref_mut() {
                let next = s.advance(chunk, Some(self));
                self.spu_counter = next;
            }
            clocks -= chunk;
        }
    }
    fn run_pending(&mut self) {
        let mut next = 0;
        if self.pending_phase >= 2 {
            self.begin_results();
            next = self.command_part2(self.pending_cmd)
        } else {
            match self.pending_phase {
                -1 => {
                    if self.args_rp != self.args_wp {
                        self.args_latch = self.args[(self.args_rp & 15) as usize];
                        self.args_rp = (self.args_rp + 1) & 31;
                        self.pending_phase += 1;
                        next = 1815
                    } else {
                        self.pending_phase += 2;
                        next = 8500
                    }
                }
                0 => {
                    if self.args_in < 32 {
                        self.args_recv[self.args_in] = self.args_latch;
                        self.args_in += 1
                    }
                    if self.args_rp != self.args_wp {
                        self.args_latch = self.args[(self.args_rp & 15) as usize];
                        self.args_rp = (self.args_rp + 1) & 31;
                        next = 1815
                    } else {
                        self.pending_phase += 1;
                        next = 8500
                    }
                }
                _ => {
                    self.begin_results();
                    if let Some((min, max)) = commands::cmd_args(self.pending_cmd) {
                        if self.args_in < min || self.args_in > max {
                            self.error(0x20)
                        } else {
                            let args = self.args_recv;
                            next = self.command(self.pending_cmd, &args[..self.args_in]);
                            self.pending_phase = 2
                        }
                    } else {
                        self.error(0x40)
                    }
                    self.args_in = 0
                }
            }
        }
        if next == 0 {
            self.pending = 0
        } else {
            self.pending += next
        }
    }
    fn sector_time(&self) -> i64 {
        CPU_HZ / if self.mode & 128 != 0 { 150 } else { 75 }
    }
    fn play_read(&mut self) {
        let Some(d) = self.disc.clone() else { return };
        let p = self.pipe_pos;
        if self.cur_sector >= d.lead_out() + 300 && self.cur_sector >= 75 * 60 * 75 - 150 {
            self.seek_finished = -1;
            self.drive = STOPPED;
            self.pipe_pos = 0;
            self.pipe_in = 0;
            self.sectors_read = 0;
            return;
        }
        if self.pipe_in >= 2 {
            let buf = self.pipe[p];
            let q = self.pipe_q[p];
            self.pipe_in -= 1;
            if q[0] & 64 != 0 {
                if self.drive == SEEKING_LOGICAL2
                    || self.drive == READING
                    || (self.hold_logical && (self.drive == PAUSED || self.drive == STANDBY))
                {
                    self.header.copy_from_slice(&buf[12..24]);
                    self.header_valid = true;
                    if self.drive == SEEKING_LOGICAL2 {
                        let lba = unbcd(self.header[0]) * 4500
                            + unbcd(self.header[1]) * 75
                            + unbcd(self.header[2])
                            - 150;
                        if lba == self.seek_target {
                            self.drive = self.after_seek;
                            self.seek_finished = 1;
                            self.report_startup = 24000000
                        } else if self.seek_retry == 0 {
                            self.hold_logical = false;
                            self.drive = STANDBY;
                            self.seek_finished = -1
                        } else {
                            self.seek_retry -= 1
                        }
                    }
                }
                if self.drive == READING {
                    if self.mode & 64 != 0 && buf[15] == 2 && buf[18] & 0x64 == 0x64 {
                        if self.xa_test(&buf) && self.audio.read >= self.audio.size {
                            self.xa_process(&buf)
                        }
                    } else {
                        let (mut off, mut size) = (12, 2048);
                        if self.mode & 32 != 0 {
                            off = 0;
                            size = 2340
                        }
                        if self.mode & 16 != 0 {
                            off = 12;
                            size = 2328
                        }
                        self.sb[..size].copy_from_slice(&buf[12 + off..12 + off + size]);
                        self.sb_in = size;
                        let s = self.status(false);
                        self.set_aip(1, &[s]);
                    }
                }
            } else {
                if self.drive == SEEKING_LOGICAL2 {
                    if self.mode & 1 != 0 {
                        self.drive = self.after_seek;
                        self.seek_finished = 1;
                        self.report_startup = 24000000
                    } else if self.seek_retry == 0 {
                        self.hold_logical = false;
                        self.drive = STANDBY;
                        self.seek_finished = -1
                    } else {
                        self.seek_retry -= 1
                    }
                }
                if ((self.drive == READING && self.mode & 1 != 0) || self.drive == PLAYING)
                    && self.audio.read >= self.audio.size
                {
                    self.buffer_cdda(&buf)
                }
            }
        }
        d.read_sector(self.cur_sector, &mut self.pipe[p]);
        self.subq = d.subq(self.cur_sector);
        self.pipe_q[p] = self.subq;
        if self.subq[1] == 0xaa
            && (self.drive == PLAYING
                || (self.drive == READING && self.subq[0] & 64 == 0 && self.mode & 1 != 0))
        {
            self.header_valid = false;
            self.seek_target = self.cur_sector;
            self.drive = PAUSED;
            self.pipe_pos = 0;
            self.pipe_in = 0;
            self.sectors_read = 0;
            let s = self.status(false);
            self.set_aip(4, &[s]);
            return;
        }
        if self.drive == PLAYING {
            if self.mode & 2 != 0 {
                if self.track_match == -1 {
                    self.track_match = self.subq[1] as i32
                }
                if self.track_match != -1 && self.track_match != self.subq[1] as i32 {
                    let s = self.status(false);
                    self.set_aip(4, &[s]);
                    self.seek_target = self.cur_sector;
                    self.drive = PAUSED;
                    self.pipe_pos = 0;
                    self.pipe_in = 0;
                    self.sectors_read = 0;
                    self.psr = 0;
                    return;
                }
            }
            if self.mode & 4 != 0
                && self.report_startup <= 0
                && (self.subq[9] >> 4 != self.report_last || self.forward || self.backward)
            {
                let mut r = [0; 8];
                r[0] = self.status(false);
                r[1] = self.subq[1];
                r[2] = self.subq[2];
                if self.subq[9] & 16 != 0 {
                    r[3] = self.subq[3];
                    r[4] = self.subq[4] | 128;
                    r[5] = self.subq[5]
                } else {
                    r[3..6].copy_from_slice(&self.subq[7..10])
                }
                self.set_aip(1, &r)
            }
            self.report_last = self.subq[9] >> 4;
        }
        self.pipe_pos = (p + 1) % 2;
        self.pipe_in += 1;
        self.psr += self.sector_time();
        if self.drive == PLAYING {
            if self.forward {
                self.cur_sector += 12
            } else if self.backward {
                self.cur_sector = (self.cur_sector - 12).max(0)
            } else {
                self.cur_sector += 1
            }
        } else {
            self.cur_sector += 1
        }
        if self.drive == PAUSED || self.drive == STANDBY {
            let add = if self.hold_logical { 2 } else { 0 };
            if self.cur_sector >= self.seek_target + add {
                self.cur_sector = (self.cur_sector - 9).max(-150)
            }
        } else {
            self.sectors_read = self.sectors_read.wrapping_add(1)
        }
    }
    fn xa_test(&self, b: &[u8; SECTOR_SIZE]) -> bool {
        self.mode & 64 != 0
            && b[18] & 4 != 0
            && (self.mode & 8 == 0 || (b[16] == self.filter_file && b[17] == self.filter_chan))
    }
    pub fn next_event(&self) -> i64 {
        if self.aip_irq != 0 && self.ready <= 0 {
            return 1;
        }
        let mut n = -1;
        for v in [
            self.psr,
            self.pending,
            self.startup,
            if self.irq_buffer & 15 == 0 {
                self.ready
            } else {
                0
            },
        ] {
            if v > 0 && (n < 0 || v < n) {
                n = v
            }
        }
        n
    }
}
