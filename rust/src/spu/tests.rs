use super::*;
fn state_hash(s: &Spu) -> u64 {
    let mut h = 14695981039346656037u64;
    let mut put = |v: u64| {
        for b in v.to_le_bytes() {
            h ^= b as u64;
            h = h.wrapping_mul(1099511628211)
        }
    };
    macro_rules! w{($($v:expr),* $(,)?)=>{{$(put($v as i64 as u64);)*}}}
    macro_rules! a {
        ($v:expr) => {{
            for &x in $v {
                w!(x)
            }
        }};
    }
    for v in &s.voices {
        a!(&v.decode);
        w!(
            v.m2,
            v.m1,
            v.delay,
            v.wp,
            v.rp,
            v.avail,
            v.ignore_loop,
            v.shift,
            v.weight,
            v.flags
        );
        for sw in &v.sweep {
            w!(sw.control, sw.current, sw.divider)
        }
        w!(
            v.pitch,
            v.phase,
            v.start_addr,
            v.cur_addr,
            v.adsr_control,
            v.loop_addr,
            v.pre_lr
        );
        let a = &v.adsr;
        w!(
            a.env,
            a.divider,
            a.phase,
            a.attack_exp,
            a.sustain_exp,
            a.sustain_dec,
            a.release_exp,
            a.attack_rate,
            a.decay_rate,
            a.sustain_rate,
            a.release_rate,
            a.sustain_level
        )
    }
    w!(
        s.noise_divider,
        s.noise_counter,
        s.lfsr,
        s.fm_mode,
        s.noise_mode,
        s.reverb_mode,
        s.reverb_wa
    );
    for sw in &s.global_sweep {
        w!(sw.control, sw.current, sw.divider)
    }
    a!(&s.reverb_vol);
    a!(&s.cd_vol);
    a!(&s.extern_vol);
    w!(
        s.irq_addr,
        s.rw_addr,
        s.control,
        s.voice_on,
        s.voice_off,
        s.block_end,
        s.cwa
    );
    a!(&s.regs);
    a!(&s.aux);
    for row in &s.rdsb {
        a!(row)
    }
    for row in &s.rusb {
        a!(row)
    }
    w!(s.rvb_pos, s.reverb_cur, s.irq, s.clock_divider, s.buf_pos);
    h
}
struct Tape {
    b: Vec<u8>,
    p: usize,
}
impl Tape {
    fn open(name: &str, magic: &[u8]) -> Option<Self> {
        let Ok(root) = std::env::var("PSGOLEM_RUST_SPU_FIXTURES") else {
            eprintln!("SKIP: select fixed Go SPU fixtures");
            return None;
        };
        let b = std::fs::read(std::path::Path::new(&root).join(name)).unwrap();
        assert_eq!(&b[..8], magic);
        Some(Self { b, p: 8 })
    }
    fn u32(&mut self) -> u32 {
        let n = u32::from_le_bytes(self.b[self.p..self.p + 4].try_into().unwrap());
        self.p += 4;
        n
    }
    fn u64(&mut self) -> u64 {
        let n = u64::from_le_bytes(self.b[self.p..self.p + 8].try_into().unwrap());
        self.p += 8;
        n
    }
    fn sample(&mut self) -> i16 {
        let n = i16::from_le_bytes(self.b[self.p..self.p + 2].try_into().unwrap());
        self.p += 2;
        n
    }
}
#[derive(Default)]
struct Ramp {
    n: u32,
}
impl CdAudio for Ramp {
    fn cd_audio_sample(&mut self) -> (i32, i32) {
        self.n = self.n.wrapping_add(1);
        (
            (self.n.wrapping_mul(1237).wrapping_add(17)) as u16 as i16 as i32,
            (self.n.wrapping_mul(997).wrapping_add(32769)) as u16 as i16 as i32,
        )
    }
}
#[test]
fn fixed_go_spu_events_samples_and_ram() {
    let Some(mut t) = Tape::open("rust-spu-oracle-v1.bin", b"PSGSPU01") else {
        return;
    };
    let n = t.u32();
    let mut s = Spu::new();
    let mut cd = Ramp::default();
    let mut total = 0;
    let mut checks = 0;
    for i in 0..n {
        let (k, a, v) = (t.u32(), t.u32(), t.u32());
        let mut samples = Vec::new();
        let result = match k {
            0 => {
                s.write(a, v as u16);
                0
            }
            1 => s.read(a) as u64,
            2 => s.advance(a as i64, if v == 0 { None } else { Some(&mut cd) }) as u64,
            3 => {
                s.dma_write(v);
                0
            }
            4 => s.dma_read() as u64,
            5 => {
                samples = s.take_samples();
                0
            }
            6 => {
                s = Spu::new();
                0
            }
            7 => 0,
            _ => panic!("bad event"),
        };
        assert_eq!(result, t.u64(), "SPU event {i} op {k} address {a:x}");
        assert_eq!(
            state_hash(&s),
            t.u64(),
            "SPU full scalar/voice/reverb state event {i} op {k} {a:x}"
        );
        assert_eq!(cd.n, t.u32(), "CD consumed count event {i}");
        let len = t.u32() as usize;
        assert_eq!(samples.len(), len, "sample count {i}");
        for sample in samples {
            assert_eq!(sample, t.sample(), "sample at event {i}");
            total += 1
        }
        if k == 7 {
            checks += 1;
            for &r in &s.ram {
                assert_eq!(r, t.sample() as u16, "full SPU RAM at event {i}")
            }
        }
    }
    assert_eq!(t.p, t.b.len());
    eprintln!(
        "RUST_SPU_GO: {n} events, {} exact stereo frames, {checks} complete RAM checkpoints",
        total / 2
    );
}
#[test]
fn fixed_go_cd_spu_clock_and_samples() {
    let Some(mut t) = Tape::open("rust-spu-cd-oracle-v1.bin", b"PSGSPC01") else {
        return;
    };
    let root = std::env::var("PSGOLEM_RUST_SPU_FIXTURES").unwrap();
    let d = crate::disc::Disc::open(
        std::path::Path::new(&root).join("rust-cdrom-fixtures-v1/rust-cdrom-fixture-v1.cue"),
    )
    .unwrap();
    let mut c = crate::cdrom::Cdrom::new();
    c.insert(Some(d));
    let mut s = Spu::new();
    for (a, v) in [
        (0x180, 0x3fff),
        (0x182, 0x3fff),
        (0x1b0, 0x7fff),
        (0x1b2, 0x7fff),
        (0x1aa, 0xc001),
    ] {
        s.write(a, v)
    }
    let n = t.u32();
    let mut frames = 0;
    let mut nonzero = 0;
    for i in 0..n {
        let (k, a, v) = (t.u32(), t.u32(), t.u32());
        if k == 0 {
            c.write(a, v as u8)
        } else {
            c.tick_with_spu(a as i64, &mut s)
        }
        let samples = s.take_samples();
        assert_eq!(samples.len(), t.u32() as usize, "event {i} sample count");
        frames += samples.len() / 2;
        for sample in samples {
            assert_eq!(sample, t.sample(), "CD/SPU event {i}");
            if sample != 0 {
                nonzero += 1
            }
        }
        assert_eq!(state_hash(&s), t.u64(), "CD/SPU state event {i}");
        assert_eq!(c.spu_counter() as u64, t.u64());
        assert_eq!(c.next_event() as u64, t.u64());
        for x in c.audio_clock_state() {
            assert_eq!(x, t.u64(), "CD clock state event {i}")
        }
    }
    assert_eq!(t.p, t.b.len());
    assert!(
        nonzero > 0,
        "CD integration must produce genuine decoded nonzero samples"
    );
    eprintln!(
        "RUST_SPU_CD_GO: {n} events, {frames} exact stereo frames, {nonzero} nonzero samples"
    );
}
#[test]
fn partition_clock_buffer_cap_and_clone() {
    let mut whole = Spu::new();
    let mut split = whole.clone();
    whole.advance(768 * 5000, None);
    for _ in 0..5000 {
        split.advance(768, None);
    }
    assert_eq!(state_hash(&whole), state_hash(&split));
    let all = whole.take_samples();
    assert_eq!(all.len(), 8192);
    assert_eq!(all, split.take_samples());
    assert!(whole.take_samples().is_empty());
    let mut clone = whole.clone();
    clone.write(0x1a6, 0x200);
    clone.dma_write(0x12345678);
    assert_ne!(whole.ram(), clone.ram());
    assert_eq!(whole.advance(100, None), 668);
    eprintln!(
        "RUST_SPU_LOCAL: split clock, 4096-frame bounded buffer, draining and deep clone passed"
    );
}
