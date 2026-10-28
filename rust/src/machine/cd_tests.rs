use super::*;
#[test]
fn cd_irq_bus_width_and_dma3() {
    let mut m = Machine::new(&vec![0; 0x80000]).unwrap();
    let b = &mut m.bus;
    b.write8(0x1f801800, 1);
    b.write32(0x1f801802, 31);
    b.write8(0x1f801800, 0);
    b.write16(0x1f801802, 0x20);
    b.write32(0x1f801801, 0x19);
    // CD registers use exactly one byte even for 16/32-bit CPU accesses.
    b.advance(100000);
    assert_eq!(b.irq_registers().0 & 4, 4);
    assert_eq!(b.read32(0x1f801801), 0x97);
    assert_eq!(b.read16(0x1f801801), 1);
    b.write8(0x1f801800, 1);
    b.write8(0x1f801803, 31);
    b.advance(2001);
    b.write32(0x1f801070, !4);
    assert_eq!(b.irq_registers().0 & 4, 0);
    let Ok(root) = std::env::var("PSGOLEM_RUST_CD_FIXTURES") else {
        eprintln!("SKIP DMA3: select fixed synthetic CD fixture");
        return;
    };
    let d = crate::disc::Disc::open(
        std::path::Path::new(&root).join("rust-cdrom-fixtures-v1/rust-cdrom-fixture-v1.cue"),
    )
    .unwrap();
    b.cd.insert(Some(d));
    b.cd.tick(33868800);
    let cmd = |c: &mut crate::cdrom::Cdrom, op: u8, args: &[u8]| {
        c.write(0, 1);
        c.write(3, 0x5f);
        c.tick(2001);
        c.write(0, 0);
        for &a in args {
            c.write(2, a)
        }
        c.write(1, op);
        c.tick(100000);
        c.write(0, 1);
        c.write(3, 31);
        c.tick(2001);
    };
    cmd(&mut b.cd, 14, &[0]);
    cmd(&mut b.cd, 2, &[0, 2, 0]);
    cmd(&mut b.cd, 6, &[]);
    b.cd.tick(5000000);
    b.cd.write(0, 0);
    b.cd.write(3, 128);
    let mut reference = b.cd.clone();
    let want: Vec<_> = (0..512).map(|_| reference.dma_read()).collect();
    assert!(want.iter().any(|&x| x != 0));
    b.write32(0x1f8010f0, 0x0765c321);
    b.write32(0x1f8010f4, 0x00880000);
    b.write32(0x1f8010b0, 0x1ffffc);
    b.write32(0x1f8010b4, 0x00020200);
    b.write32(0x1f8010b8, 0x01000200);
    // 2 blocks of 512 words; second half exhausts FIFO and returns zero.
    for (i, &w) in want.iter().enumerate() {
        assert_eq!(b.read32((0x1ffffc + 4 * i as u32) & 0x1ffffc), w)
    }
    assert_eq!(b.read32(0x1f8010b0), 0xffc);
    assert_eq!(b.read32(0x1f8010b4), 512);
    assert_eq!(b.read32(0x1f8010b8) & 0x11000000, 0);
    assert_ne!(b.read32(0x1f8010f4) & 0x88000000, 0);
    assert_ne!(b.irq_registers().0 & 8, 0);
    eprintln!("RUST_CD_MACHINE: byte-wide CD CPU accesses, IRQ2/ACK and DMA3 RAM wrap/MADR/BCR/DICR passed");
}
#[test]
fn real_bios_disc_executable_entry() {
    let Ok(cue) = std::env::var("PSGOLEM_RUST_REAL_CUE") else {
        eprintln!("SKIP: select genuine SOTN disc and BIOS");
        return;
    };
    let bios = std::env::var("PSGOLEM_RUST_CD_BIOS").expect("Genuine BIOS required");
    let d = crate::disc::Disc::open(cue).unwrap();
    assert_eq!(d.boot_file(), "SLPM_860.23");
    assert_eq!(d.region(), "SCEI");
    let exe = d.read_file(d.boot_file()).unwrap();
    assert_eq!(&exe[..8], b"PS-X EXE");
    let entry = u32::from_le_bytes(exe[16..20].try_into().unwrap());
    let dest = u32::from_le_bytes(exe[24..28].try_into().unwrap());
    let size = u32::from_le_bytes(exe[28..32].try_into().unwrap());
    let mut m = Machine::new(&std::fs::read(bios).unwrap()).unwrap();
    m.insert_disc(d);
    m.run_until_pc(entry, 800000000).unwrap();
    let a = (physical(dest) & 0x1fffff) as usize;
    assert_eq!(
        &m.bus.ram()[a..a + size as usize],
        &exe[0x800..0x800 + size as usize]
    );
    assert!(!String::from_utf8_lossy(m.tty()).contains("VSync: timeout"));
    eprintln!("RUST_REAL_DISC_BOOT: normal reset/BIOS, PC={entry:08x}, cycles={}, field={}, complete executable body {size} bytes exact",m.cycles(),m.field());
}
