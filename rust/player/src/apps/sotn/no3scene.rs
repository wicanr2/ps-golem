//! NO3 typed tile/actor/HUD composition, 026 §16.26, GPL-3.0-or-later.
//! Complete source component only; no live player registration or guest CPU.
use super::{
    actor::ActorPlan, actorview::ActorView, call::Tile, fieldhud::Clip, hud::Hud, no3work::Work,
    tileview::View,
};
use psgolem_core::{gpu::source::VramSource, scene::Canvas};
use std::path::Path;
type Result<T> = std::result::Result<T, String>;
pub struct Bundle {
    pub work: Work,
    pub tiles: View,
    pub actors: ActorView,
    pub hud: Hud,
    pub clip: Clip,
}
impl Bundle {
    pub fn prepare(
        r: &[u8],
        scratch: &[u8],
        v: &(impl VramSource + ?Sized),
        gfx: &[u8],
        hud_archive: &[u8],
        tiles: &[Tile],
        actors: &ActorPlan,
        actor_seed: &[u8],
        pack: &Path,
    ) -> Result<Self> {
        let work = Work::prepare(r, scratch, tiles, &actors.quads, actor_seed)?;
        let tiles = View::prepare_no3(r, v, gfx, tiles, pack)?;
        let actors = ActorView::prepare_at(r, v, actors, pack, 0, 0)?;
        let hud = Hud::prepare(r, v, Hud::source(hud_archive)?, pack)?;
        if hud.deferred_records != 2
            || hud.records.len() != work.hud
            || actors.orders.iter().any(|o| *o >= 400)
        {
            return Err("NO3可見來源或跨層排序不完整".into());
        }
        let clip = Clip::read(r)?;
        Ok(Self {
            work,
            tiles,
            actors,
            hud,
            clip,
        })
    }
    pub fn render(&self, scale: usize) -> Result<Canvas> {
        if ![1, 4].contains(&scale) {
            return Err("NO3共同畫布倍率未封閉".into());
        }
        let mut c = Canvas {
            width: 256 * scale,
            height: 240 * scale,
            words: vec![self.clip.background; 256 * 240 * scale * scale],
        };
        self.tiles.paint_background(&mut c, scale)?;
        self.tiles.paint_foreground(&mut c, scale)?;
        self.actors.paint(&mut c, scale)?;
        self.hud.paint(&mut c, scale)?;
        self.clip.apply(&mut c, scale)?;
        Ok(c)
    }
}
