# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@AGENTS.md

## Claude 專屬補充

- 動手前依 `~/.claude/rules/00-rules-index.md` 觸發表載入規則。本專案常用：
  skill `re-retro-cht-rulebook`、`~/.claude/knowledge-base/retro/retro-remake-spec-gated-workflow.md`、
  `~/cht/dosgolem/CLAUDE.md`（方法論來源）。
- HD 實作交給 Codex（`codex` CLI）。Claude 負責規格、驗收與對拍；派工 prompt 要寫明
  `AGENTS.md` 的硬規則與禁止的收尾動作（commit、push、docker 清理）。
