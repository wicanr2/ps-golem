//! Native button script compatibility, 010 / 026 section12.
//! GPL-3.0-or-later. Port of internal/padscript/padscript.go at 417fbcb.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Press {
    pub at: u64,
    pub duration: u64,
    pub buttons: u16,
}
const NAMES: [(&str, u16); 14] = [
    ("SELECT", 1),
    ("START", 8),
    ("UP", 16),
    ("RIGHT", 32),
    ("DOWN", 64),
    ("LEFT", 128),
    ("L2", 256),
    ("R2", 512),
    ("L1", 1024),
    ("R1", 2048),
    ("TRIANGLE", 4096),
    ("CIRCLE", 8192),
    ("CROSS", 16384),
    ("SQUARE", 32768),
];
pub fn parse_buttons(s: &str) -> Result<u16, String> {
    if s.is_empty() || s == "-" {
        return Ok(0);
    }
    let mut result = 0;
    for name in s.split('+') {
        let upper = name.to_uppercase();
        result |= NAMES
            .iter()
            .find(|(n, _)| *n == upper)
            .ok_or_else(|| format!("Unknown button {name:?}"))?
            .1;
    }
    Ok(result)
}
pub fn format_buttons(buttons: u16) -> String {
    let mut names: Vec<&str> = NAMES
        .iter()
        .filter(|(_, v)| buttons & v != 0)
        .map(|(n, _)| *n)
        .collect();
    names.sort();
    names.join("+")
}
fn number(s: &str) -> Result<u64, String> {
    if s.is_empty() || !s.bytes().all(|v| v.is_ascii_digit()) {
        return Err(format!("Invalid field number {s:?}"));
    }
    s.parse()
        .map_err(|_| format!("Field number exceeds u64: {s:?}"))
}
pub fn parse(s: &str) -> Result<Vec<Press>, String> {
    let mut out = Vec::new();
    if s.trim().is_empty() {
        return Ok(out);
    }
    for item in s.trim().split(',') {
        let parts: Vec<&str> = item.trim().split(':').collect();
        if parts.len() < 2 {
            return Err(format!("Expected FIELD:BTN[:DUR]: {item:?}"));
        }
        out.push(Press {
            at: number(parts[0])?,
            buttons: parse_buttons(parts[1])?,
            duration: if parts.len() > 2 {
                number(parts[2])?
            } else {
                5
            },
        });
    }
    out.sort_by_key(|p| p.at);
    Ok(out)
}
pub fn at(script: &[Press], field: u64) -> u16 {
    let mut buttons = 0;
    for p in script {
        if p.at > field {
            break;
        }
        if field < p.at.wrapping_add(p.duration) {
            buttons |= p.buttons;
        }
    }
    buttons
}
#[cfg(test)]
mod tests {
    use super::*;
    struct Tape {
        b: Vec<u8>,
        i: usize,
    }
    impl Tape {
        fn n<const N: usize>(&mut self) -> [u8; N] {
            let v = self.b[self.i..self.i + N].try_into().unwrap();
            self.i += N;
            v
        }
        fn u32(&mut self) -> u32 {
            u32::from_le_bytes(self.n())
        }
        fn u64(&mut self) -> u64 {
            u64::from_le_bytes(self.n())
        }
        fn u16(&mut self) -> u16 {
            u16::from_le_bytes(self.n())
        }
        fn text(&mut self) -> String {
            let n = self.u32() as usize;
            let s = String::from_utf8(self.b[self.i..self.i + n].to_vec()).unwrap();
            self.i += n;
            s
        }
    }
    #[test]
    fn fixed_go_parser_and_field_boundaries() {
        let Ok(path) = std::env::var("PSGOLEM_RUST_INPUT_ORACLE") else {
            eprintln!("SKIP: select fixed Go input oracle");
            return;
        };
        let mut t = Tape {
            b: std::fs::read(path).unwrap(),
            i: 0,
        };
        assert_eq!(&t.n::<8>(), b"PSGINP01");
        let cases = t.u32();
        let mut queries = 0;
        for i in 0..cases {
            let source = t.text();
            let valid = t.u32() != 0;
            let got = parse(&source);
            assert_eq!(got.is_ok(), valid, "script {i}: {source:?}");
            if !valid {
                continue;
            }
            let script = got.unwrap();
            let count = t.u32() as usize;
            assert_eq!(script.len(), count);
            for p in &script {
                assert_eq!(
                    (p.at, p.duration, p.buttons),
                    (t.u64(), t.u64(), t.u16()),
                    "script {i} press"
                );
            }
            for _ in 0..t.u32() {
                let field = t.u64();
                assert_eq!(at(&script, field), t.u16(), "script {i} field{field}");
                queries += 1;
            }
        }
        let masks = t.u32();
        for _ in 0..masks {
            let bits = t.u16();
            let text = t.text();
            assert_eq!(format_buttons(bits), text);
            assert_eq!(parse_buttons(&text).unwrap(), bits & 0xfff9);
        }
        assert_eq!(t.i, t.b.len());
        eprintln!("RUST_INPUT: {cases} Go parser cases, {queries} field boundaries, {masks} formatted masks; full M4 script included");
    }
}
