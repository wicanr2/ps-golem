use super::*;
#[test]
fn fixed_go_pad_card_and_ack_events() {
    let Ok(path) = std::env::var("PSGOLEM_SIO_ORACLE") else {
        eprintln!("SKIP: select the fixed public Go SIO oracle");
        return;
    };
    let data = std::fs::read(path).unwrap();
    assert_eq!(&data[..8], b"PSGSIO01");
    let mut pos = 8;
    fn word(data: &[u8], pos: &mut usize) -> u32 {
        let v = u32::from_le_bytes(data[*pos..*pos + 4].try_into().unwrap());
        *pos += 4;
        v
    }
    fn long(data: &[u8], pos: &mut usize) -> u64 {
        let v = u64::from_le_bytes(data[*pos..*pos + 8].try_into().unwrap());
        *pos += 8;
        v
    }
    let count = word(&data, &mut pos);
    assert!(count > 10000);
    let mut sio = Sio::new();
    sio.pads = [Some(Gamepad::default()), Some(Gamepad::default())];
    sio.cards = [Some(Memcard::new()), Some(Memcard::new())];
    for event in 0..count {
        let kind = word(&data, &mut pos);
        let a = word(&data, &mut pos);
        let value = word(&data, &mut pos);
        let result = match kind {
            0 => {
                sio.write(a, value);
                0
            }
            1 => sio.read(a),
            2 => {
                sio.tick(a as i64);
                0
            }
            3 => {
                sio.pads[a as usize].as_mut().unwrap().buttons = value as u16;
                0
            }
            4 => {
                sio.cards[a as usize].as_mut().unwrap().dirty = value != 0;
                0
            }
            _ => panic!("event"),
        };
        assert_eq!(result, word(&data, &mut pos), "SIO RX event {event}");
        assert_eq!(
            sio.next_event() as u64,
            long(&data, &mut pos),
            "SIO next-event {event}"
        );
        for (field, value) in sio.state_words().into_iter().enumerate() {
            assert_eq!(
                value,
                long(&data, &mut pos),
                "SIO event {event}, field {field}"
            );
        }
        for c in sio.cards.iter().flatten() {
            assert_eq!(c.dirty as u64, long(&data, &mut pos));
            assert_eq!(c.writes, long(&data, &mut pos));
        }
    }
    for c in sio.cards.iter().flatten() {
        assert_eq!(c.data.as_ref().as_slice(), &data[pos..pos + (1 << 17)]);
        pos += 1 << 17;
    }
    assert_eq!(pos, data.len());
    assert_eq!(sio.cards[0].as_ref().unwrap().writes, 2);
    assert!(!sio.cards[0].as_ref().unwrap().dirty);
    assert_eq!(sio.cards[1].as_ref().unwrap().writes, 1);
    eprintln!("SIO_GO_ORACLE: {count} register/bit/ACK events, two 131072-byte cards exact");
}
#[test]
fn card_load_length_format_and_clone_ownership() {
    let mut card = Memcard::new();
    let before = card.clone();
    assert!(!card.load(&[1; 128]));
    assert_eq!(card.data, before.data);
    for frame in 0..16 {
        let data = &card.data[frame * 128..(frame + 1) * 128];
        assert_eq!(data[..127].iter().fold(0, |a, b| a ^ b), data[127]);
    }
    let mut bytes = card.data.clone();
    bytes[127] = 0xab;
    assert!(card.load(bytes.as_ref()));
    assert!(!card.dirty);
    let mut machine = crate::machine::Machine::new(&vec![0; 0x80000]).unwrap();
    machine.bus.sio.cards[0]
        .as_mut()
        .unwrap()
        .load(bytes.as_ref());
    let mut cloned = machine.clone();
    cloned.bus.sio.cards[0].as_mut().unwrap().data[127] = 0xcd;
    assert_eq!(machine.bus.sio.cards[0].as_ref().unwrap().data[127], 0xab);
}

#[test]
fn machine_sio_irq_and_byte_register_path() {
    use crate::cpu::Bus;
    let mut m = crate::machine::Machine::new(&vec![0; 0x80000]).unwrap();
    m.bus.write16(0x1f80104e, 32);
    m.bus.write16(0x1f80104a, 0xc03);
    m.bus.read8(0x1f801040);
    m.bus.write8(0x1f801040, 1);
    m.run(256);
    let b = &mut m.bus;
    assert_eq!(b.irq_registers().0 & (1 << 7), 1 << 7);
    assert_eq!(b.read8(0x1f801040), 255);
    b.write16(0x1f80104a, 0x13);
    assert!(!b.sio.irq());
    b.write32(0x1f801070, !(1 << 7));
    assert_eq!(b.irq_registers().0 & (1 << 7), 0);
}
