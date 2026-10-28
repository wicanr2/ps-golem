use super::*;
fn machine() -> Machine {
    Machine::new(&vec![0; 0x80000]).unwrap()
}
#[test]
fn memory_aliases_scratch_isolation_and_fetch_errors() {
    let mut m = machine();
    let b = &mut m.bus;
    b.write32(0x80000100, 0x76543210);
    for p in [0x100, 0x200100, 0x80000100, 0xa0000100] {
        assert_eq!(b.read32(p), 0x76543210);
    }
    b.write16(0x9f800010, 0xabcd);
    assert_eq!(b.read16(0x1f800010), 0xabcd);
    assert_eq!(b.read16(0xbf800010), 0);
    b.write32(0xfffe0130, 0x80);
    b.isolated_write(0x20, 0x12345678, 4);
    assert_eq!(b.read32(0x1f800020), 0x12345678);
    assert!(b.instruction_bus_error(0x9f800000));
    assert!(b.instruction_bus_error(0x1f801070));
    assert!(b.instruction_bus_error(0x1f801820));
    assert!(!b.instruction_bus_error(0x1f801c00));
    assert_eq!(b.read_cost(0xbfc00000, 4), 24);
    assert_eq!(b.read_cost(0x9f800000, 4), 0);
}
#[test]
fn dma_otc_full_source_dicr_and_linked_gpu() {
    let mut m = machine();
    let b = &mut m.bus;
    b.write32(0x1f8010f0, 0x0f654b21);
    b.write32(0x1f8010f4, 0x00c00000);
    b.write32(0x1f8010e0, 0x110);
    b.write32(0x1f8010e4, 4);
    b.write32(0x1f8010e8, 0x11000002);
    assert_eq!(b.read32(0x110), 0x10c);
    assert_eq!(b.read32(0x104), 0xffffff);
    assert_eq!(b.read32(0x1f8010f4) & 0xc0000000, 0xc0000000);
    // A byte write uses the full source register; it must not re-ACK latched flag bits.
    b.write8(0x1f8010f6, 0xc0);
    assert_eq!(b.read32(0x1f8010f4) & 0x40000000, 0x40000000);
    b.write8(0x1f8010f6, 0x40c0);
    assert_eq!(b.read32(0x1f8010f4) & 0x40000000, 0);
    b.write32(0x200, 0x01800000);
    b.write32(0x204, 0xe300140a);
    b.write32(0x1f8010a0, 0x200);
    b.write32(0x1f8010a8, 0x01000401);
    b.write32(0x1f801814, 0x10000003);
    assert_eq!(b.read32(0x1f801810) & 0xfffff, 0x140a);
    assert_eq!(b.read32(0x1f8010a0), 0xffffff);
}
#[test]
fn timer_event_flag_clear_irq_and_gpu_clock() {
    let mut m = machine();
    let b = &mut m.bus;
    b.write32(0x1f801104, 1 << 4);
    b.write32(0x1f801108, 100);
    b.advance(100);
    assert_eq!(b.irq_registers().0 & (1 << 4), 1 << 4);
    assert_ne!(b.read32(0x1f801104) & (1 << 11), 0);
    assert_eq!(b.read32(0x1f801104) & (1 << 11), 0);
    b.write32(0x1f801074, 1 << 4);
    assert!(b.irq_pending());
    b.write16(0x1f801070, !(1 << 4));
    assert!(!b.irq_pending());
    let mut whole = machine();
    let mut split = whole.clone();
    whole.bus.advance(1000000);
    for _ in 0..1000 {
        split.bus.advance(1000);
    }
    assert_eq!(whole.bus.gpu.video_state(), split.bus.gpu.video_state());
    assert_eq!(whole.bus.irq_registers(), split.bus.irq_registers());
    assert_eq!(whole.field(), split.field());
}
#[test]
fn exe_truncation_and_normal_register_setup() {
    let mut m = machine();
    assert!(m.sideload_exe(&[0; 0x800]).is_err());
    let mut exe = vec![0; 0x804];
    exe[..8].copy_from_slice(b"PS-X EXE");
    for (off, v) in [
        (0x10, 0x80010000u32),
        (0x14, 0x8001abc0),
        (0x18, 0x80010000),
        (0x1c, 4),
        (0x30, 0x801fff00),
        (0x34, 16),
    ] {
        exe[off..off + 4].copy_from_slice(&v.to_le_bytes());
    }
    exe[0x800..].copy_from_slice(&0x24020001u32.to_le_bytes());
    m.sideload_exe(&exe).unwrap();
    assert_eq!(m.cpu.pc(), 0x80010000);
    assert_eq!(m.cpu.gpr(28), 0x8001abc0);
    assert_eq!(m.cpu.gpr(29), 0x801fff10);
    m.step();
    assert_eq!(m.cpu.gpr(2), 1);
}

fn normalized(bytes: &[u8]) -> Vec<String> {
    String::from_utf8_lossy(bytes)
        .replace('\r', "")
        .lines()
        .map(|line| line.strip_prefix("% ").unwrap_or(line))
        .filter(|line| !line.trim().is_empty())
        .map(str::to_owned)
        .collect()
}
#[test]
fn real_bios_public_cpu_programs() {
    let Ok(bios_dir) = std::env::var("PSGOLEM_RUST_BIOS_DIR") else {
        eprintln!("SKIP: select genuine BIOS directory");
        return;
    };
    let root = std::path::PathBuf::from(
        std::env::var("PSGOLEM_RUST_MACHINE_FIXTURES")
            .expect("Select extracted pinned public test fixtures"),
    );
    let mut total = 0;
    for bios_name in ["SCPH5500.BIN", "SCPH-100.BIN"] {
        let bios = std::fs::read(std::path::Path::new(&bios_dir).join(bios_name))
            .expect("Selected genuine BIOS is required");
        let mut boot = Machine::new(&bios).unwrap();
        boot.run_until_pc(SHELL_ENTRY, 400000000).unwrap();
        eprintln!(
            "RUST_BIOS_BOOT: {bios_name}, cycles={}, field={}, PC={:08x}",
            boot.cycles(),
            boot.field(),
            boot.cpu.pc()
        );
        assert!(!String::from_utf8_lossy(boot.tty()).contains("VSync: timeout"));
        for test in ["cpu/cop", "cpu/code-in-io", "gte/test-all"] {
            let name = test.rsplit('/').next().unwrap();
            let dir = root.join(test);
            let exe = std::fs::read(dir.join(format!("{name}.exe"))).unwrap();
            let want = normalized(&std::fs::read(dir.join("psx.log")).unwrap());
            assert!(!want.is_empty());
            let mut m = boot.clone();
            let mark = m.tty().len();
            m.sideload_exe(&exe).unwrap();
            m.run(100000000);
            let mut got = normalized(&m.tty()[mark..]);
            // The single previously documented program/log drift, 006 §7.
            if test == "gte/test-all" {
                if let Some(i) = got.iter().position(|line| line == "Total tests: 1150") {
                    got.remove(i);
                }
            }
            let start = got
                .iter()
                .position(|line| line == &want[0])
                .unwrap_or_else(|| {
                    panic!("{bios_name} {test}: no expected header; actual TTY {got:?}")
                });
            assert_eq!(&got[start..], want.as_slice(), "{bios_name} {test}");
            eprintln!(
                "RUST_CPU_PROGRAM: {bios_name} {test}, {} exact TTY lines",
                want.len()
            );
            total += 1;
        }
    }
    assert_eq!(total, 6);
    eprintln!("RUST_MACHINE_PUBLIC_CPU: 2 genuine BIOS boots, 6 programs, all exact TTY comparisons passed");
}
