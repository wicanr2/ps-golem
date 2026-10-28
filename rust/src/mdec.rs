//! Native MDEC, 011 / 026 section11. GPL-3.0-or-later.
//! Port of internal/mdec/mdec.go at b7f80d4, itself derived from Beetle
//! mednafen/psx/mdec.c e57860d82faa953dc0fc10f201fd583739bc2f7d,
//! GPL-2.0-or-later. Keeps the existing clock and arithmetic approximations.
#[derive(Clone, Default)]
struct Fifo {
    data: [u32; 32],
    read: u32,
    write: u32,
    len: u32,
}
impl Fifo {
    fn space(&self) -> u32 {
        32 - self.len
    }
    fn flush(&mut self) {
        self.read = 0;
        self.write = 0;
        self.len = 0;
    }
    fn push(&mut self, v: u32) {
        self.data[self.write as usize] = v;
        self.write = (self.write + 1) & 31;
        self.len += 1;
    }
    fn pop(&mut self) -> u32 {
        let v = self.data[self.read as usize];
        self.read = (self.read + 1) & 31;
        self.len -= 1;
        v
    }
}
#[derive(Clone)]
pub struct Mdec {
    input: Fifo,
    output: Fifo,
    y: [i8; 64],
    cb: [i8; 64],
    cr: [i8; 64],
    control: u32,
    command: u32,
    in_command: bool,
    state: u32,
    quant: [[u8; 64]; 2],
    quant_index: u32,
    idct_matrix: [i16; 64],
    idct_index: u32,
    scale: u8,
    coeff: [i16; 64],
    coeff_index: u32,
    decode_block: u32,
    pixels: [u8; 192],
    pixel_read: u32,
    pixel_words: u32,
    pending_pixels: bool,
    input_counter: u16,
    last_encoded: u32,
    clock: i32,
    wait_clock: bool,
    command_pending: bool,
    offset_y: u8,
    offset_counter: u8,
    offset_width: u8,
}
const ZIGZAG: [usize; 64] = [
    0x00, 0x08, 0x01, 0x02, 0x09, 0x10, 0x18, 0x11, 0x0a, 0x03, 0x04, 0x0b, 0x12, 0x19, 0x20, 0x28,
    0x21, 0x1a, 0x13, 0x0c, 0x05, 0x06, 0x0d, 0x14, 0x1b, 0x22, 0x29, 0x30, 0x38, 0x31, 0x2a, 0x23,
    0x1c, 0x15, 0x0e, 0x07, 0x0f, 0x16, 0x1d, 0x24, 0x2b, 0x32, 0x39, 0x3a, 0x33, 0x2c, 0x25, 0x1e,
    0x17, 0x1f, 0x26, 0x2d, 0x34, 0x3b, 0x3c, 0x35, 0x2e, 0x27, 0x2f, 0x36, 0x3d, 0x3e, 0x37, 0x3f,
];
fn sign10(v: u16) -> i32 {
    ((v as i32) << 22) >> 22
}
fn clamp9(v: i32) -> i8 {
    ((v << 23) >> 23).clamp(-128, 127) as i8
}
fn rgb(y: i8, cb: i8, cr: i8) -> [u8; 3] {
    let (y, cb, cr) = (y as i32, cb as i32, cr as i32);
    [
        clamp9(y + ((359 * cr + 128) >> 8)) as u8 ^ 128,
        clamp9(y + (((-88 * cb & !31) + (-183 * cr & !7) + 128) >> 8)) as u8 ^ 128,
        clamp9(y + ((454 * cb + 128) >> 8)) as u8 ^ 128,
    ]
}
impl Default for Mdec {
    fn default() -> Self {
        Self::new()
    }
}
impl Mdec {
    pub fn new() -> Self {
        Self {
            input: Fifo::default(),
            output: Fifo::default(),
            y: [0; 64],
            cb: [0; 64],
            cr: [0; 64],
            control: 0,
            command: 0,
            in_command: false,
            state: 0,
            quant: [[0; 64]; 2],
            quant_index: 0,
            idct_matrix: [0; 64],
            idct_index: 0,
            scale: 0,
            coeff: [0; 64],
            coeff_index: 0,
            decode_block: 0,
            pixels: [0; 192],
            pixel_read: 0,
            pixel_words: 0,
            pending_pixels: false,
            input_counter: 0,
            last_encoded: 0,
            clock: 0,
            wait_clock: false,
            command_pending: false,
            offset_y: 0,
            offset_counter: 0,
            offset_width: 0,
        }
    }
    fn idct(&self) -> [i8; 64] {
        let mut tmp = [0i16; 64];
        let mut out = [0i8; 64];
        for col in 0..8 {
            for x in 0..8 {
                let mut sum = 0i32;
                for u in 0..8 {
                    sum = sum.wrapping_add(
                        (self.coeff[col * 8 + u] as i32)
                            .wrapping_mul(self.idct_matrix[x * 8 + u] as i32),
                    );
                }
                tmp[x * 8 + col] = (sum.wrapping_add(0x4000) >> 15) as i16;
            }
        }
        for col in 0..8 {
            for x in 0..8 {
                let mut sum = 0i32;
                for u in 0..8 {
                    sum = sum.wrapping_add(
                        (tmp[col * 8 + u] as i32).wrapping_mul(self.idct_matrix[x * 8 + u] as i32),
                    );
                }
                out[col * 8 + x] = clamp9(sum.wrapping_add(0x4000) >> 15);
            }
        }
        out
    }
    fn encode(&mut self, block: u32) {
        self.pixel_words = 0;
        let signed = self.command & (1 << 26) != 0;
        match (self.command >> 27) & 3 {
            0 => {
                let xor = if signed { 0 } else { 0x88 };
                for i in 0..32 {
                    let a = (self.y[i * 2] as i32 + 8).min(127) as u8;
                    let b = (self.y[i * 2 + 1] as i32 + 8).min(127) as u8;
                    self.pixels[i] = ((a >> 4) | (b & 0xf0)) ^ xor;
                }
                self.pixel_words = 8;
            }
            1 => {
                let xor = if signed { 0 } else { 128 };
                for i in 0..64 {
                    self.pixels[i] = self.y[i] as u8 ^ xor;
                }
                self.pixel_words = 16;
            }
            format => {
                let mut offset = 0;
                for y in 0..8 {
                    for x in 0..8 {
                        let cy = (y >> 1) | ((block as usize & 2) << 1);
                        let cx = ((block as usize & 1) << 2) + (x >> 1);
                        let [r, g, b] = rgb(
                            self.y[y * 8 + x],
                            self.cb[cy * 8 + cx],
                            self.cr[cy * 8 + cx],
                        );
                        if format == 2 {
                            let xor = if signed { 128 } else { 0 };
                            self.pixels[offset..offset + 3].copy_from_slice(&[
                                r ^ xor,
                                g ^ xor,
                                b ^ xor,
                            ]);
                            offset += 3;
                        } else {
                            let c = |v: u8| ((v as u16 + 4) >> 3).min(31);
                            let mut xor = if signed { 0x4210 } else { 0 };
                            if self.command & 0x02000000 != 0 {
                                xor |= 0x8000;
                            }
                            let word = (c(r) | (c(g) << 5) | (c(b) << 10)) ^ xor;
                            self.pixels[offset..offset + 2].copy_from_slice(&word.to_le_bytes());
                            offset += 2;
                        }
                    }
                }
                self.pixel_words = if format == 2 { 48 } else { 32 };
            }
        }
    }
    fn image_data(&mut self, v: u16) -> i32 {
        let q = if self.decode_block < 2 { 1 } else { 0 };
        let round = |c: i32| {
            if c < 0 {
                8
            } else if c > 0 {
                -8
            } else {
                0
            }
        };
        if self.coeff_index == 0 {
            if v == 0xfe00 {
                return 0;
            }
            self.scale = (v >> 10) as u8;
            let quant = self.quant[q][0] as i32;
            let c = sign10(v & 1023);
            let t = if quant != 0 {
                (c * quant).wrapping_shl(4) + round(c)
            } else {
                (c * 2).wrapping_shl(4)
            };
            self.coeff[ZIGZAG[0]] = t.clamp(-0x4000, 0x3fff) as i16;
            self.coeff_index += 1;
        } else if v == 0xfe00 {
            while self.coeff_index < 64 {
                self.coeff[ZIGZAG[self.coeff_index as usize]] = 0;
                self.coeff_index += 1;
            }
        } else {
            for _ in 0..v >> 10 {
                if self.coeff_index >= 64 {
                    break;
                }
                self.coeff[ZIGZAG[self.coeff_index as usize]] = 0;
                self.coeff_index += 1;
            }
            if self.coeff_index < 64 {
                let quant = self.scale as i32 * self.quant[q][self.coeff_index as usize] as i32;
                let c = sign10(v & 1023);
                let t = if quant != 0 {
                    ((c * quant) >> 3).wrapping_shl(4) + round(c)
                } else {
                    (c * 2).wrapping_shl(4)
                };
                self.coeff[ZIGZAG[self.coeff_index as usize]] = t.clamp(-0x4000, 0x3fff) as i16;
                self.coeff_index += 1;
            }
        }
        if self.coeff_index != 64 {
            return 0;
        }
        self.coeff_index = 0;
        let block = self.idct();
        match self.decode_block {
            0 => self.cr = block,
            1 => self.cb = block,
            _ => self.y = block,
        }
        if self.decode_block >= 2 {
            self.last_encoded = (self.decode_block + 4) % 6;
            self.encode(self.last_encoded);
        }
        self.decode_block += 1;
        let color = (self.command >> 27) & 2 != 0;
        if color && self.decode_block == 6 {
            self.decode_block = 0;
        } else if !color && self.decode_block == 3 {
            self.decode_block = 2;
        }
        512
    }
    pub fn waiting_clock(&self) -> bool {
        self.wait_clock
    }
    pub fn tick(&mut self, n: i32) {
        self.clock = self.clock.wrapping_add(n).min(128);
        self.run();
    }
    fn end_command(&mut self) {
        self.state = 0;
        self.in_command = false;
    }
    fn start_command(&mut self, c: u32) {
        self.command = c;
        self.in_command = true;
        match (c >> 29) & 7 {
            1 => {
                self.input_counter = c as u16;
                self.output.flush();
                self.pixel_words = 0;
                self.coeff_index = 0;
                self.decode_block = if (c >> 27) & 2 != 0 { 0 } else { 2 };
                self.offset_width = match (c >> 27) & 3 {
                    2 => 6,
                    3 => 4,
                    _ => 0,
                };
                self.offset_y = 0;
                self.offset_counter = self.offset_width;
                self.input_counter = self.input_counter.wrapping_sub(1);
                self.state = 1;
            }
            2 => {
                self.quant_index = 0;
                self.input_counter = if c & 1 != 0 { 31 } else { 15 };
                self.state = 2;
            }
            3 => {
                self.idct_index = 0;
                self.input_counter = 31;
                self.state = 3;
            }
            _ => {
                self.input_counter = c as u16;
                self.in_command = false;
            }
        }
    }
    fn run(&mut self) {
        loop {
            if self.wait_clock {
                if self.clock <= 0 {
                    return;
                }
                self.wait_clock = false;
                if self.command_pending {
                    self.command_pending = false;
                    self.start_command(self.command);
                    continue;
                }
            }
            if self.pending_pixels {
                while self.pixel_read < self.pixel_words {
                    if self.output.space() == 0 {
                        return;
                    }
                    let i = self.pixel_read as usize * 4;
                    self.output.push(u32::from_le_bytes(
                        self.pixels[i..i + 4].try_into().unwrap(),
                    ));
                    self.pixel_read += 1;
                }
                self.pending_pixels = false;
                if self.input_counter == 65535 {
                    self.end_command();
                }
            }
            if self.input.len == 0 {
                return;
            }
            match self.state {
                0 => {
                    self.command = self.input.pop();
                    self.in_command = true;
                    self.clock = self.clock.wrapping_sub(1);
                    self.command_pending = true;
                    self.wait_clock = true;
                }
                1 => {
                    let w = self.input.pop();
                    self.input_counter = self.input_counter.wrapping_sub(1);
                    self.pixel_words = 0;
                    let mut eat = self.image_data(w as u16);
                    eat += self.image_data((w >> 16) as u16);
                    self.pixel_read = 0;
                    self.pending_pixels = true;
                    self.clock = self.clock.wrapping_sub(eat);
                    self.wait_clock = true;
                }
                2 => {
                    let mut w = self.input.pop();
                    self.input_counter = self.input_counter.wrapping_sub(1);
                    for _ in 0..4 {
                        self.quant[(self.quant_index >> 6) as usize]
                            [(self.quant_index & 63) as usize] = w as u8;
                        self.quant_index = (self.quant_index + 1) & 127;
                        w >>= 8;
                    }
                    if self.input_counter == 65535 {
                        self.end_command();
                    }
                }
                3 => {
                    let mut w = self.input.pop();
                    self.input_counter = self.input_counter.wrapping_sub(1);
                    for _ in 0..2 {
                        let i = ((self.idct_index & 7) << 3) | ((self.idct_index >> 3) & 7);
                        self.idct_matrix[i as usize] = (w as i16) >> 3;
                        self.idct_index = (self.idct_index + 1) & 63;
                        w >>= 16;
                    }
                    if self.input_counter == 65535 {
                        self.end_command();
                    }
                }
                _ => unreachable!(),
            }
        }
    }
    pub fn dma_write(&mut self, v: u32) {
        if self.input.space() != 0 {
            self.input.push(v);
            self.run();
        }
    }
    pub fn dma_read(&mut self) -> (u32, u32) {
        if self.output.len == 0 {
            return (0, 0);
        }
        let v = self.output.pop();
        let mut offset = (self.offset_y as u32 & 7) * self.offset_width as u32;
        if self.offset_y & 8 != 0 {
            offset = offset.wrapping_sub(self.offset_width as u32 * 7);
        }
        self.offset_counter = self.offset_counter.wrapping_sub(1);
        if self.offset_counter == 0 {
            self.offset_counter = self.offset_width;
            self.offset_y = self.offset_y.wrapping_add(1);
        }
        self.run();
        (v, offset)
    }
    pub fn dma_can_write(&self) -> bool {
        self.input.space() >= 32
            && self.control & (1 << 30) != 0
            && self.in_command
            && self.input_counter != 65535
    }
    pub fn dma_can_read(&self) -> bool {
        self.output.len >= 32 && self.control & (1 << 29) != 0
    }
    pub fn write(&mut self, address: u32, v: u32) {
        if address & 4 != 0 {
            if v & 0x80000000 != 0 {
                self.state = 0;
                self.input_counter = 0;
                self.command = 0;
                self.in_command = false;
                self.pixel_words = 0;
                self.pending_pixels = false;
                self.clock = 0;
                self.wait_clock = false;
                self.command_pending = false;
                self.quant_index = 0;
                self.idct_index = 0;
                self.scale = 0;
                self.coeff = [0; 64];
                self.coeff_index = 0;
                self.decode_block = 0;
                self.input.flush();
                self.output.flush();
            }
            self.control = v & 0x7fffffff;
            return;
        }
        if self.input.space() != 0 {
            self.input.push(v);
            if !self.in_command && self.clock < 1 {
                self.clock = 1;
            }
            self.run();
        }
    }
    pub fn status(&self) -> u32 {
        let mut r = 0;
        if self.output.len == 0 {
            r |= 1 << 31;
        }
        if self.input.space() == 0 {
            r |= 1 << 30;
        }
        if self.in_command {
            r |= 1 << 29;
        }
        if self.dma_can_write() {
            r |= 1 << 28;
        }
        if self.dma_can_read() {
            r |= 1 << 27;
        }
        let block = if (self.command >> 27) & 2 == 0 {
            4
        } else if self.output.len != 0 || self.pending_pixels {
            self.last_encoded
        } else {
            (self.decode_block + 4) % 6
        };
        r | (((self.command >> 25) & 15) << 23) | (block << 16) | self.input_counter as u32
    }
    pub fn read(&mut self, address: u32) -> u32 {
        if address & 4 != 0 {
            return self.status();
        }
        if self.output.len == 0 {
            return 0;
        }
        let v = self.output.pop();
        self.run();
        v
    }
    #[cfg(test)]
    pub(crate) fn oracle_hash(&self) -> u64 {
        let mut h = 14695981039346656037u64;
        let mut add = |mut n: u64| {
            for _ in 0..8 {
                h ^= n & 255;
                h = h.wrapping_mul(1099511628211);
                n >>= 8;
            }
        };
        for f in [&self.input, &self.output] {
            for &v in &f.data {
                add(v as u64);
            }
            add(f.read as u64);
            add(f.write as u64);
            add(f.len as u64);
        }
        for b in [&self.y, &self.cb, &self.cr] {
            for &v in b {
                add(v as i64 as u64);
            }
        }
        for v in [
            self.control as u64,
            self.command as u64,
            self.in_command as u64,
            self.state as u64,
        ] {
            add(v);
        }
        for b in self.quant {
            for v in b {
                add(v as u64);
            }
        }
        add(self.quant_index as u64);
        for v in self.idct_matrix {
            add(v as i64 as u64);
        }
        add(self.idct_index as u64);
        add(self.scale as u64);
        for v in self.coeff {
            add(v as i64 as u64);
        }
        add(self.coeff_index as u64);
        add(self.decode_block as u64);
        for v in self.pixels {
            add(v as u64);
        }
        for v in [
            self.pixel_read as u64,
            self.pixel_words as u64,
            self.pending_pixels as u64,
            self.input_counter as u64,
            self.last_encoded as u64,
            self.clock as i64 as u64,
            self.wait_clock as u64,
            self.command_pending as u64,
            self.offset_y as u64,
            self.offset_counter as u64,
            self.offset_width as u64,
        ] {
            add(v);
        }
        h
    }
}
#[cfg(test)]
mod tests;
