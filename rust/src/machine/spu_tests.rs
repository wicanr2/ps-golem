use super::*;
#[test]
fn spu_cpu_width_dma4_irq_and_audio() {
    let mut m = Machine::new(&vec![0; 0x80000]).unwrap();
    let b = &mut m.bus;
    b.write16(0x1f801daa, 0xc000);
    b.write8(0x1f801dab, 0x12);
    assert_eq!(b.read16(0x1f801daa), 0xc000);
    b.write8(0x1f801daa, 0x76ab);
    assert_eq!(b.read16(0x1f801daa), 0x76ab);
    b.write32(0x1f801d80, 0x12345678);
    assert_eq!(b.read32(0x1f801d80), 0x12345678);
    b.write32(0x100, 0x89abcdef);
    b.write32(0x104, 0x01234567);
    b.write16(0x1f801da6, 0x200);
    b.write16(0x1f801da4, 0x200);
    b.write16(0x1f801daa, 0xc060);
    b.write32(0x1f8010f0, 0x076d4321);
    b.write32(0x1f8010f4, 0x00900000);
    b.write32(0x1f8010c0, 0x100);
    b.write32(0x1f8010c4, 0x00010002);
    b.write32(0x1f8010c8, 0x01000201);
    b.advance(1);
    assert_ne!(b.irq_registers().0 & (1 << 9), 0);
    assert_ne!(b.irq_registers().0 & 8, 0);
    assert_eq!(b.read32(0x1f8010c0), 0x108);
    assert_eq!(b.read32(0x1f8010c4), 2);
    assert_eq!(b.spu.ram()[0x800..0x804], [0xcdef, 0x89ab, 0x4567, 0x0123]);
    b.write16(0x1f801daa, 0xc030);
    b.write16(0x1f801da6, 0x200);
    b.write32(0x1f8010c0, 0x200);
    b.write32(0x1f8010c4, 2);
    b.write32(0x1f8010c8, 0x01000000);
    assert_eq!(b.read32(0x200), 0x89abcdef);
    assert_eq!(b.read32(0x204), 0x01234567);
    b.advance(768 * 10 - 1);
    assert_eq!(m.take_audio().len(), 20);
    assert!(m.take_audio().is_empty());
    eprintln!("RUST_SPU_MACHINE: 8/16/32-bit bus, DMA4 both directions/MADR/BCR, IRQ9/DMA IRQ and TakeAudio passed");
}

#[test]
fn real_bios_public_spu_programs() {
    let Ok(bios) = std::env::var("PSGOLEM_RUST_SPU_BIOS") else {
        eprintln!("SKIP: select genuine BIOS and public SPU fixtures");
        return;
    };
    let root = std::path::PathBuf::from(
        std::env::var("PSGOLEM_RUST_SPU_FIXTURES").expect("select fixed SPU fixtures"),
    )
    .join("rust-spu-public-v1");
    let mut boot = Machine::new(&std::fs::read(bios).unwrap()).unwrap();
    boot.run_until_pc(SHELL_ENTRY, 400000000).unwrap();
    for name in ["memory-transfer", "stereo", "test"] {
        let mut m = boot.clone();
        let mark = m.tty().len();
        m.take_audio();
        m.sideload_exe(&std::fs::read(root.join(format!("{name}.exe"))).unwrap())
            .unwrap();
        m.run(100000000);
        assert_eq!(
            &m.tty()[mark..],
            std::fs::read(root.join(format!("{name}.tty")))
                .unwrap()
                .as_slice(),
            "{name} exact Go TTY"
        );
        let pcm: Vec<u8> = m
            .take_audio()
            .iter()
            .flat_map(|v| v.to_le_bytes())
            .collect();
        assert_eq!(
            pcm,
            std::fs::read(root.join(format!("{name}.pcm"))).unwrap(),
            "{name} complete PCM"
        );
        let ram: Vec<u8> = m
            .bus
            .spu
            .ram()
            .iter()
            .flat_map(|v| v.to_le_bytes())
            .collect();
        assert_eq!(
            ram,
            std::fs::read(root.join(format!("{name}.spu-ram"))).unwrap(),
            "{name} complete SPU RAM"
        );
        eprintln!("RUST_SPU_PUBLIC: {name}, cycles={}, field={}, PC={:08x}, exact Go TTY/PCM/full SPU RAM",m.cycles(),m.field(),m.cpu.pc());
    }
}
