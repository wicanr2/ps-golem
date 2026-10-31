//! Own native Rust machine and devices, 026. Player/HD frontend still pending.
#![forbid(unsafe_code)]
pub mod gpu;

pub mod gte;

pub mod cpu;

pub mod machine;

pub mod sio;

pub mod cdrom;
pub mod disc;

pub mod spu;

pub mod movie;

pub mod mdec;

pub mod input;
pub mod cardfile;

pub mod scene;

pub mod paint;
