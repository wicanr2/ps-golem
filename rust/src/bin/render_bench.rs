//! Linux-only CPU/wall timer shell. Rendering is safe, standalone Rust.
use psgolem_core::gpu::Gpu;
use std::fs::File;
use std::io::{self, BufRead, BufReader, Read, Write};
use std::time::Instant;

#[cfg(not(all(target_os = "linux", target_arch = "x86_64")))]
compile_error!("benchmark CPU timer is pinned to Linux x86_64; core crate remains portable");
// libc clock(), only for measurement. No foreign rendering code is called.
unsafe extern "C" {
    fn clock() -> i64;
}
fn cpu_ns() -> i64 {
    unsafe { clock() * 1000 }
}
fn issue(g: &mut Gpu, v: u64) {
    match v >> 32 {
        0 => {
            g.gp0(v as u32);
            g.drain_readback();
        }
        1 => g.gp1(v as u32),
        2 => g.set_field(v & 1 != 0),
        9 => (),
        _ => panic!("tag"),
    }
}
fn digest(g: &Gpu) -> u64 {
    let mut h = 14695981039346656037u64;
    for &w in g.vram() {
        for b in w.to_le_bytes() {
            h = (h ^ b as u64).wrapping_mul(1099511628211);
        }
    }
    h
}
fn main() -> io::Result<()> {
    let path = std::env::args().nth(1).expect("stream path");
    let mut gpu = Gpu::new();
    let mut scenes: Vec<Option<Gpu>> = vec![None, None];
    let mut input = BufReader::with_capacity(1 << 20, File::open(&path)?);
    let mut buf = [0u8; 8];
    loop {
        match input.read_exact(&mut buf) {
            Ok(()) => (),
            Err(e) if e.kind() == io::ErrorKind::UnexpectedEof => break,
            Err(e) => return Err(e),
        };
        let v = u64::from_le_bytes(buf);
        issue(&mut gpu, v);
        if v >> 32 == 9 {
            for (i, field) in [11000, 18000].into_iter().enumerate() {
                if v as u32 == field {
                    scenes[i] = Some(gpu.clone());
                }
            }
        }
    }
    let mut words = Vec::new();
    for i in 0..2 {
        assert!(scenes[i].is_some());
        let b = std::fs::read(format!("{path}.scene{i}"))?;
        assert_eq!(b.len() % 8, 0);
        words.push(
            b.chunks_exact(8)
                .map(|w| u64::from_le_bytes(w.try_into().unwrap()))
                .collect::<Vec<_>>(),
        );
    }
    println!("{{\"ready\":true,\"engine\":\"ps-golem-Rust-native\"}}");
    io::stdout().flush()?;
    for line in io::stdin().lock().lines() {
        let text = line?;
        let index: usize = match text.trim().parse() {
            Ok(n) if n < 2 => n,
            _ => break,
        };
        let mut g = scenes[index].as_ref().unwrap().clone();
        let c0 = cpu_ns();
        let start = Instant::now();
        for &v in &words[index] {
            issue(&mut g, v);
        }
        let ns = start.elapsed().as_nanos();
        let cpu = cpu_ns() - c0;
        let mut raw = Vec::with_capacity(1024 * 512 * 2);
        for &w in g.vram() {
            raw.extend_from_slice(&w.to_le_bytes());
        }
        std::fs::write(format!("{path}.rust-after{index}"), raw)?;
        println!("{{\"scene\":{index},\"elapsed_ns\":{ns},\"cpu_ns\":{cpu},\"fields\":120,\"VRAM_fnv64\":\"{:x}\"}}",digest(&g));
        io::stdout().flush()?;
    }
    Ok(())
}
