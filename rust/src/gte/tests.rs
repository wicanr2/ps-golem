use super::*;

#[test]
fn hardware_corpus_1150_all_64_registers() {
    let path = if let Ok(root) = std::env::var("PSGOLEM_PS1TESTS") {
        std::path::PathBuf::from(root).join("gte-fuzz/gte_valid_0xc0ffee_50.log")
    } else {
        let candidate = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../testdata/ps1-tests/gte-fuzz/gte_valid_0xc0ffee_50.log");
        if !candidate.is_file() {
            eprintln!(
                "SKIP: fetch the pinned public ps1-tests corpus with tools/fetch_ps1tests.sh"
            );
            return;
        }
        candidate
    };
    let log = std::fs::read_to_string(path)
        .expect("Pinned hardware corpus must exist when explicitly selected");
    struct Case {
        number: usize,
        input: [u32; 64],
        output: [u32; 64],
        written: [bool; 64],
        read: [bool; 64],
        cmd: Option<u32>,
    }
    impl Case {
        fn new(number: usize) -> Self {
            Self {
                number,
                input: [0; 64],
                output: [0; 64],
                written: [false; 64],
                read: [false; 64],
                cmd: None,
            }
        }
        fn check(&self) {
            assert!(
                self.written.iter().all(|v| *v) && self.read.iter().all(|v| *v),
                "Incomplete corpus case {}",
                self.number
            );
            let mut g = Gte::new();
            for (i, &v) in self.input.iter().enumerate() {
                g.write(i, v);
            }
            if let Some(cmd) = self.cmd {
                g.execute(cmd);
            }
            for (i, &want) in self.output.iter().enumerate() {
                assert_eq!(g.read(i), want, "Test {}, reg {}", self.number, i);
            }
        }
    }
    let mut case = None;
    let mut count = 0;
    for line in log.lines() {
        if let Some(number) = line.strip_prefix("Test ") {
            if let Some(previous) = case.take() {
                let previous: Case = previous;
                previous.check();
                count += 1;
            }
            case = Some(Case::new(number.parse().unwrap()));
            continue;
        }
        let Some(current) = case.as_mut() else {
            continue;
        };
        if line.starts_with("> r[") || line.starts_with("< r[") {
            let (index, value) = line[4..].split_once("] = 0x").unwrap();
            let i: usize = index.parse().unwrap();
            assert!(i < 64);
            let value = u32::from_str_radix(value, 16).unwrap();
            if line.starts_with('>') {
                assert!(!current.written[i]);
                current.written[i] = true;
                current.input[i] = value;
            } else {
                assert!(!current.read[i]);
                current.read[i] = true;
                current.output[i] = value;
            }
        } else if line.starts_with("GTE 0x") {
            let parts: Vec<_> = line.split_whitespace().collect();
            assert_eq!(parts.len(), 8);
            let op = u32::from_str_radix(&parts[1][2..], 16).unwrap();
            let f: Vec<u32> = parts[3..]
                .iter()
                .map(|part| {
                    part.split_once('=')
                        .unwrap()
                        .1
                        .trim_matches(|c: char| !c.is_ascii_digit())
                        .parse()
                        .unwrap()
                })
                .collect();
            current.cmd = Some(
                0x4a000000
                    | (op & 63)
                    | (f[1] << 10)
                    | (f[2] << 13)
                    | (f[3] << 15)
                    | (f[4] << 17)
                    | (f[0] << 19),
            );
        }
    }
    if let Some(last) = case {
        last.check();
        count += 1;
    }
    assert_eq!(count, 1150);
    eprintln!("HARDWARE_GTE: 1150 cases x 64 registers = 73600 exact reads");
}

#[test]
fn ir0_upper_bound_and_summary_flag() {
    let mut g = Gte::new();
    assert_eq!(g.lim_h(4096), 4096);
    assert_eq!(g.flags, 0);
    assert_eq!(g.lim_h(4097), 4096);
    assert_eq!(g.flags, 1 << 12);
    g.write(63, 0xffffffff);
    assert_eq!(g.read(63), 0xfffff000);
    g.write(63, 1 << 12);
    assert_eq!(g.read(63), 1 << 12);
    g.write(63, 1 << 24);
    assert_eq!(g.read(63), (1 << 31) | (1 << 24));
}

#[test]
fn execute_latency_and_register_fifo_boundaries() {
    let mut g = Gte::new();
    g.write(12, 1);
    g.write(13, 2);
    g.write(14, 3);
    g.write(15, 4);
    assert_eq!(
        [g.read(12), g.read(13), g.read(14), g.read(15)],
        [2, 3, 4, 4]
    );
    g.write(30, 0xffffffff);
    assert_eq!(g.read(31), 32);
    g.write(30, 0);
    assert_eq!(g.read(31), 32);
    for (cmd, cycles) in [
        (1, 15),
        (6, 8),
        (0x12, 8),
        (0x16, 44),
        (0x30, 23),
        (0x3f, 39),
    ] {
        assert_eq!(g.execute(cmd), cycles);
    }
}
