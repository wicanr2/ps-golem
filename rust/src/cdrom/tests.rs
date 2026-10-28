use super::*;
fn snapshot(c: &Cdrom) -> u64 {
    let mut h = 14695981039346656037u64;
    let mut put = |v: u64| {
        for b in v.to_le_bytes() {
            h ^= b as u64;
            h = h.wrapping_mul(1099511628211)
        }
    };
    macro_rules! w{($($x:expr),* $(,)?)=>{{$(put($x as i64 as u64);)*}}}
    macro_rules! arr {
        ($x:expr) => {
            for &v in $x {
                w!(v)
            }
        };
    }
    w!(c.is_psx);
    arr!(&c.disc_id);
    w!(c.drive, c.disc_changed, c.startup, c.selector);
    arr!(&c.args);
    w!(c.args_wp, c.args_rp, c.args_latch);
    arr!(&c.args_recv);
    w!(c.args_in);
    arr!(&c.results);
    w!(c.results_in, c.results_wp, c.results_rp);
    arr!(&c.dma);
    w!(c.dma_pos);
    arr!(&c.sb);
    w!(c.sb_in);
    for row in &c.pipe {
        arr!(row)
    }
    for row in &c.pipe_q {
        arr!(row)
    }
    w!(c.pipe_pos, c.pipe_in);
    arr!(&c.subq);
    arr!(&c.header);
    w!(
        c.header_valid,
        c.irq_buffer,
        c.irq_mask,
        c.ready,
        c.filter_file,
        c.filter_chan,
        c.pending_cmd,
        c.pending_phase,
        c.pending,
        c.mode,
        c.after_seek,
        c.forward,
        c.backward,
        c.muted,
        c.track_match,
        c.psr,
        c.cur_sector,
        c.sectors_read,
        c.aip_irq
    );
    arr!(&c.aip_res);
    w!(
        c.aip_count,
        c.seek_target,
        c.seek_retry,
        c.seek_finished,
        c.hold_logical,
        c.report_startup,
        c.command_loc,
        c.loc_dirty,
        c.report_last
    );
    let a = &c.audio;
    for row in &a.samples {
        arr!(row)
    }
    w!(a.size, a.freq, a.read);
    for row in &a.prev {
        arr!(row)
    }
    for row in &a.resamp {
        arr!(row)
    }
    w!(a.pos, a.phase);
    for row in &a.pending_vol {
        arr!(row)
    }
    for row in &a.vol {
        arr!(row)
    }
    w!(c.spu_counter);
    h
}
#[test]
fn fixed_go_cdrom_disc_and_audio_events() {
    let Ok(root) = std::env::var("PSGOLEM_RUST_CD_FIXTURES") else {
        eprintln!("SKIP: select fixed Go CD oracle");
        return;
    };
    let root = std::path::Path::new(&root);
    let d = Disc::open(root.join("rust-cdrom-fixtures-v1/rust-cdrom-fixture-v1.cue")).unwrap();
    assert!(d.fallback.is_some());
    assert_eq!(d.region(), "SCEI");
    assert_eq!(d.boot_file(), "SLPM_000.00");
    let bytes = std::fs::read(root.join("rust-cdrom-oracle-v1.bin")).unwrap();
    assert_eq!(&bytes[..8], b"PSGCDR01");
    let count = u32::from_le_bytes(bytes[8..12].try_into().unwrap());
    let mut pos = 12;
    let mut c = Cdrom::new();
    for event in 0..count {
        let mut word = || {
            let v = u32::from_le_bytes(bytes[pos..pos + 4].try_into().unwrap());
            pos += 4;
            v
        };
        let (k, a, v) = (word(), word(), word());
        let mut long = || {
            let v = u64::from_le_bytes(bytes[pos..pos + 8].try_into().unwrap());
            pos += 8;
            v
        };
        let (want, next, hash) = (long(), long() as i64, long());
        let result = match k {
            0 => {
                c.write(a, v as u8);
                0
            }
            1 => c.read(a) as u64,
            2 => {
                c.tick(a as i64);
                0
            }
            3 => c.dma_read() as u64,
            4 => {
                let mut h = 14695981039346656037u64;
                for _ in 0..a {
                    let (l, r) = c.cd_audio_sample();
                    for v in [l, r] {
                        for b in v.to_le_bytes() {
                            h ^= b as u64;
                            h = h.wrapping_mul(1099511628211)
                        }
                    }
                }
                h
            }
            5 => {
                c.insert(if a == 0 { None } else { Some(d.clone()) });
                0
            }
            _ => panic!("Invalid event"),
        };
        assert_eq!(result, want, "event {event}: op {k} {a:x} {v:x}");
        assert_eq!(c.next_event(), next, "event {event}: next");
        assert_eq!(
            snapshot(&c),
            hash,
            "event {event}: complete controller/audio state"
        );
    }
    assert_eq!(pos, bytes.len());
    eprintln!("RUST_CDROM_GO: {count} events, complete controller/audio state hashes and sample results exact");
}
#[test]
fn fixed_go_disc_sectors_subq_iso_and_clone() {
    let Ok(root) = std::env::var("PSGOLEM_RUST_CD_FIXTURES") else {
        eprintln!("SKIP: select fixed Go disc oracle");
        return;
    };
    let root = std::path::Path::new(&root);
    let d = Disc::open(root.join("rust-cdrom-fixtures-v1/rust-cdrom-fixture-v1.cue")).unwrap();
    let data = std::fs::read(root.join("rust-disc-oracle-v1.bin")).unwrap();
    let mut raw = [0; SECTOR_SIZE];
    for rec in data.chunks_exact(4 + SECTOR_SIZE + 12) {
        let l = i32::from_le_bytes(rec[..4].try_into().unwrap());
        d.read_sector(l, &mut raw);
        assert_eq!(raw.as_slice(), &rec[4..4 + SECTOR_SIZE], "LBA {l}");
        assert_eq!(d.subq(l).as_slice(), &rec[4 + SECTOR_SIZE..], "SubQ {l}");
    }
    assert_eq!(data.len() % (4 + SECTOR_SIZE + 12), 0);
    let want = std::fs::read(root.join("rust-disc-file-v1.bin")).unwrap();
    assert_eq!(d.read_file("SUB\\file.dat;1").unwrap(), want);
    assert!(d.read_file_limit("sub/file.dat", 2999).is_err());
    for path in [
        "../x",
        "sub//file.dat",
        "sub/./file.dat",
        "SYSTEM.CNF/file.dat",
        "missing",
    ] {
        assert!(d.read_file(path).is_err(), "{path}")
    }
    let mut a = Cdrom::new();
    a.insert(Some(d));
    let mut b = a.clone();
    b.tick(CPU_HZ);
    b.write(0, 0);
    b.write(2, 0x80);
    assert_ne!(snapshot(&a), snapshot(&b));
    a.tick(CPU_HZ);
    assert_eq!(a.startup, b.startup);
    eprintln!("RUST_DISC_GO: 18 complete raw sectors/SubQ, 3000-byte ISO file, boundaries and independent controller clone passed");
}
