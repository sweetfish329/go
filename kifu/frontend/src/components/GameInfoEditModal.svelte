<script lang="ts">
  import { onMount } from 'svelte';
  import { auth } from '../lib/auth.svelte';

  interface Kifu {
    id: string;
    title: string;
    sgf_data?: string;
    is_private?: boolean;
    share_token?: string;
    black_player?: string;
    black_rank?: string;
    white_player?: string;
    white_rank?: string;
    result?: string;
    game_date?: string;
    komi?: number;
    handicap?: number;
    [key: string]: any;
  }

  let { kifu, onClose, onSuccess } = $props<{
    kifu: Kifu;
    onClose: () => void;
    onSuccess: (updated: Kifu) => void;
  }>();

  // Form states for editing
  let title = $state("");
  let blackPlayer = $state("");
  let blackRank = $state("");
  let whitePlayer = $state("");
  let whiteRank = $state("");
  let gameDate = $state(new Date().toISOString().slice(0, 10));
  let komi = $state(6.5);
  let handicap = $state(0);
  let rawResult = $state("");
  let scoreLead = $state("2.5");

  let loading = $state(false);
  let error = $state<string | null>(null);
  let modalEl = $state<HTMLElement | null>(null);
  let previousActiveElement = $state<HTMLElement | null>(null);

  const getM = () => (window as any).M;

  // Initialize form from kifu on mount
  onMount(() => {
    title = kifu.title || "";
    blackPlayer = kifu.black_player || "";
    blackRank = kifu.black_rank || "";
    whitePlayer = kifu.white_player || "";
    whiteRank = kifu.white_rank || "";
    gameDate = kifu.game_date || new Date().toISOString().slice(0, 10);
    komi = kifu.komi !== undefined ? kifu.komi : 6.5;
    handicap = kifu.handicap !== undefined ? kifu.handicap : 0;
    rawResult = kifu.result || "";

    if (rawResult.startsWith("B+") && rawResult !== "B+R" && rawResult !== "B+T") {
      scoreLead = rawResult.slice(2);
    } else if (rawResult.startsWith("W+") && rawResult !== "W+R" && rawResult !== "W+T") {
      scoreLead = rawResult.slice(2);
    }
  });

  // Result quick selection helper
  function setResultPreset(val: string) {
    if (val === "B+Score") {
      rawResult = `B+${scoreLead || "0.5"}`;
    } else if (val === "W+Score") {
      rawResult = `W+${scoreLead || "0.5"}`;
    } else {
      rawResult = val;
    }
  }

  function handleScoreLeadChange(val: string) {
    scoreLead = val;
    if (rawResult.startsWith("B+")) {
      rawResult = `B+${val}`;
    } else if (rawResult.startsWith("W+")) {
      rawResult = `W+${val}`;
    }
  }

  // Focus and keyboard accessibility
  $effect(() => {
    if (typeof document !== 'undefined') {
      previousActiveElement = document.activeElement as HTMLElement;
      const firstInput = modalEl?.querySelector('input');
      if (firstInput) {
        firstInput.focus();
      }
    }
    return () => {
      if (previousActiveElement) {
        previousActiveElement.focus();
      }
    };
  });

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      onClose();
    }
    if (e.key === 'Tab' && modalEl) {
      const focusableSelectors = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';
      const focusableElements = Array.from(modalEl.querySelectorAll(focusableSelectors)) as HTMLElement[];
      if (focusableElements.length === 0) return;

      const firstElement = focusableElements[0];
      const lastElement = focusableElements[focusableElements.length - 1];

      if (e.shiftKey) {
        if (document.activeElement === firstElement) {
          lastElement.focus();
          e.preventDefault();
        }
      } else {
        if (document.activeElement === lastElement) {
          firstElement.focus();
          e.preventDefault();
        }
      }
    }
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      onClose();
    }
  }

  async function handleSave(e: Event) {
    e.preventDefault();
    loading = true;
    error = null;

    try {
      const res = await fetch(`/api/kifus/${kifu.id}/metadata`, {
        method: 'PUT',
        headers: {
          ...auth.getHeaders(),
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          title: title.trim(),
          black_player: blackPlayer.trim(),
          black_rank: blackRank.trim(),
          white_player: whitePlayer.trim(),
          white_rank: whiteRank.trim(),
          game_date: gameDate.trim(),
          result: rawResult.trim(),
          komi: Number(komi) || 0,
          handicap: Number(handicap) || 0,
        })
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "対局情報の更新に失敗しました。");
      }

      const M = getM();
      if (M) {
        M.toast({ html: '対局情報を更新しました！', classes: 'green darken-1' });
      }

      onSuccess(data);
      onClose();
    } catch (err: any) {
      error = err.message || "更新中にエラーが発生しました。";
    } finally {
      loading = false;
    }
  }
</script>

<svelte:window onkeydown={handleKeyDown} />

<div class="modal-backdrop animate-fade-in" onclick={handleBackdropClick} role="presentation">
  <div bind:this={modalEl} class="modal-content nm-modal" role="dialog" aria-modal="true" aria-labelledby="modal-title" tabindex="-1">
    
    <!-- Slanted Collage Tag -->
    <span class="em-collage-tag" style="position: absolute; top: -14px; left: 24px; z-index: 10; font-size: 0.72rem; box-shadow: 2px 2px 0 var(--wc-shadow-solid, var(--wc-text));">
      Specs — Match Record Editor
    </span>

    <form onsubmit={handleSave}>
      <!-- Header -->
      <div class="modal-header">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <h2 id="modal-title" class="modal-title font-mincho">
            <i class="material-icons modal-title-icon" aria-hidden="true">edit_note</i>
            対局情報の編集
          </h2>
          <button type="button" class="close-icon-btn" onclick={onClose} aria-label="閉じる">
            <i class="material-icons" style="font-size: 1.25rem;">close</i>
          </button>
        </div>
        <p class="modal-subtitle">
          タイトル・対局者名・手合割・対局結果を更新し、SGFデータ本体にも即時反映します。
        </p>
      </div>

      {#if error}
        <div class="error-panel font-sans" aria-live="polite">
          <i class="material-icons error-icon" aria-hidden="true">error_outline</i>
          <span>{error}</span>
        </div>
      {/if}

      <!-- Form Body -->
      <div class="form-body">
        
        <!-- Section 1: 基本情報 (Title & Date) -->
        <div class="form-section">
          <div class="section-label font-sans">
            <i class="material-icons section-icon" aria-hidden="true">event_note</i> 基本情報
          </div>

          <div class="row-fields">
            <div class="field-group full-width">
              <label for="edit-title" class="field-label">対局名 / タイトル (GN/EV)</label>
              <input
                id="edit-title"
                type="text"
                bind:value={title}
                class="nm-input font-mincho"
                placeholder="例: 第72期 王座戦 予選"
              />
            </div>

            <div class="field-group full-width">
              <label for="edit-date" class="field-label">対局日 (DT)</label>
              <input
                id="edit-date"
                type="date"
                bind:value={gameDate}
                class="nm-input font-mono"
              />
            </div>
          </div>
        </div>

        <!-- Section 2: 対局者 (Players) -->
        <div class="form-section">
          <div class="section-label font-sans">
            <i class="material-icons section-icon" aria-hidden="true">people</i> 対局者
          </div>

          <div class="players-container">
            <!-- Black Player -->
            <div class="player-card black-player-card">
              <div class="player-header">
                <span class="stone-dot black-dot"></span>
                <span class="player-role">黒番 (PB)</span>
              </div>
              <div class="player-inputs">
                <div class="field-group name-field">
                  <label for="edit-black-player" class="field-label">対局者名</label>
                  <input
                    id="edit-black-player"
                    type="text"
                    bind:value={blackPlayer}
                    class="nm-input font-mincho"
                    placeholder="黒番対局者"
                  />
                </div>
                <div class="field-group rank-field">
                  <label for="edit-black-rank" class="field-label">段位 (BR)</label>
                  <input
                    id="edit-black-rank"
                    type="text"
                    bind:value={blackRank}
                    class="nm-input"
                    placeholder="例: 9段, 1d"
                  />
                </div>
              </div>
            </div>

            <!-- White Player -->
            <div class="player-card white-player-card">
              <div class="player-header">
                <span class="stone-dot white-dot"></span>
                <span class="player-role">白番 (PW)</span>
              </div>
              <div class="player-inputs">
                <div class="field-group name-field">
                  <label for="edit-white-player" class="field-label">対局者名</label>
                  <input
                    id="edit-white-player"
                    type="text"
                    bind:value={whitePlayer}
                    class="nm-input font-mincho"
                    placeholder="白番対局者"
                  />
                </div>
                <div class="field-group rank-field">
                  <label for="edit-white-rank" class="field-label">段位 (WR)</label>
                  <input
                    id="edit-white-rank"
                    type="text"
                    bind:value={whiteRank}
                    class="nm-input"
                    placeholder="例: 8段, 2d"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Section 3: 手合割 (Komi & Handicap) -->
        <div class="form-section">
          <div class="section-label font-sans">
            <i class="material-icons section-icon" aria-hidden="true">tune</i> 手合割 (コミ / 置き石)
          </div>

          <div class="row-fields split-2">
            <!-- Komi -->
            <div class="field-group">
              <label for="edit-komi" class="field-label">コミ (KM)</label>
              <input
                id="edit-komi"
                type="number"
                step="0.5"
                bind:value={komi}
                class="nm-input font-mono"
              />
              <!-- Quick Chips -->
              <div class="quick-chips">
                <button type="button" class="chip-btn" class:active={komi === 6.5} onclick={() => komi = 6.5}>6.5目</button>
                <button type="button" class="chip-btn" class:active={komi === 7.5} onclick={() => komi = 7.5}>7.5目</button>
                <button type="button" class="chip-btn" class:active={komi === 0} onclick={() => komi = 0}>0目</button>
              </div>
            </div>

            <!-- Handicap -->
            <div class="field-group">
              <label for="edit-handicap" class="field-label">置き石 (HA)</label>
              <select id="edit-handicap" bind:value={handicap} class="nm-select font-mono">
                <option value={0}>0子 (互先)</option>
                <option value={2}>2子</option>
                <option value={3}>3子</option>
                <option value={4}>4子</option>
                <option value={5}>5子</option>
                <option value={6}>6子</option>
                <option value={7}>7子</option>
                <option value={8}>8子</option>
                <option value={9}>9子</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Section 4: 対局結果 (Result) -->
        <div class="form-section">
          <div class="section-label font-sans">
            <i class="material-icons section-icon" aria-hidden="true">emoji_events</i> 対局結果 (RE)
          </div>

          <!-- Quick Pill Buttons -->
          <div class="result-pills">
            <button
              type="button"
              class="pill-btn"
              class:selected={rawResult === 'B+R'}
              onclick={() => setResultPreset('B+R')}
            >
              ● 黒中押し勝ち (B+R)
            </button>
            <button
              type="button"
              class="pill-btn"
              class:selected={rawResult === 'W+R'}
              onclick={() => setResultPreset('W+R')}
            >
              ○ 白中押し勝ち (W+R)
            </button>
            <button
              type="button"
              class="pill-btn"
              class:selected={rawResult.startsWith('B+') && rawResult !== 'B+R' && rawResult !== 'B+T'}
              onclick={() => setResultPreset('B+Score')}
            >
              ● 黒目数勝ち
            </button>
            <button
              type="button"
              class="pill-btn"
              class:selected={rawResult.startsWith('W+') && rawResult !== 'W+R' && rawResult !== 'W+T'}
              onclick={() => setResultPreset('W+Score')}
            >
              ○ 白目数勝ち
            </button>
            <button
              type="button"
              class="pill-btn"
              class:selected={rawResult === 'Draw'}
              onclick={() => setResultPreset('Draw')}
            >
              持碁 (Draw)
            </button>
            <button
              type="button"
              class="pill-btn"
              class:selected={rawResult === ''}
              onclick={() => setResultPreset('')}
            >
              対局中 / 未定
            </button>
          </div>

          <!-- SGF Result Input & Score Lead -->
          <div class="row-fields split-2" style="margin-top: 10px;">
            <div class="field-group">
              <label for="edit-result" class="field-label">SGF表記 (RE)</label>
              <input
                id="edit-result"
                type="text"
                bind:value={rawResult}
                class="nm-input font-mono"
                placeholder="例: B+R, W+2.5, Draw"
              />
            </div>
            {#if rawResult.startsWith('B+') && rawResult !== 'B+R' && rawResult !== 'B+T' || rawResult.startsWith('W+') && rawResult !== 'W+R' && rawResult !== 'W+T'}
              <div class="field-group">
                <label for="edit-score-lead" class="field-label">目数 (例: 2.5)</label>
                <input
                  id="edit-score-lead"
                  type="text"
                  bind:value={scoreLead}
                  oninput={(e) => handleScoreLeadChange((e.target as HTMLInputElement).value)}
                  class="nm-input font-mono"
                  placeholder="2.5"
                />
              </div>
            {/if}
          </div>
        </div>

      </div>

      <!-- Footer Actions -->
      <div class="modal-footer">
        <button type="button" class="cancel-btn" onclick={onClose} disabled={loading}>
          キャンセル
        </button>
        <button type="submit" class="save-btn" disabled={loading}>
          {#if loading}
            保存中...
          {:else}
            変更を保存
          {/if}
        </button>
      </div>

    </form>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(37, 53, 48, 0.45);
    backdrop-filter: var(--wc-glass-blur);
    -webkit-backdrop-filter: var(--wc-glass-blur);
    z-index: 1100;
    display: flex;
    justify-content: center;
    align-items: center;
    padding: 16px;
  }

  .modal-content {
    box-sizing: border-box;
    width: 100%;
    max-width: 580px;
    background: var(--wc-surface) !important;
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important;
    box-shadow: 8px 8px 0px var(--wc-shadow-dark) !important;
    padding: 32px 28px 24px 28px !important;
    max-height: calc(100vh - 40px);
    overflow-y: auto !important;
    position: relative;
  }

  .modal-header {
    margin-bottom: 20px;
    text-align: left;
  }

  .modal-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 800;
    color: var(--wc-text);
    font-size: 1.35rem;
    margin: 0;
    letter-spacing: 0.04em;
  }

  .modal-title-icon {
    font-size: 1.5rem;
    color: var(--wc-accent);
  }

  .close-icon-btn {
    border: none;
    background: none;
    color: var(--wc-text-muted);
    cursor: pointer;
    padding: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s ease;
  }

  .close-icon-btn:hover {
    color: var(--wc-text);
  }

  .modal-subtitle {
    margin: 6px 0 0 0;
    font-size: 0.8rem;
    color: var(--wc-text-muted);
    line-height: 1.5;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  .error-panel {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(160, 50, 40, 0.06);
    border: 1.5px solid #9d2f2f;
    border-radius: 0px !important;
    padding: 10px 14px;
    margin-bottom: 18px;
    color: #9d2f2f;
    font-size: 0.85rem;
  }

  .error-icon {
    font-size: 1.1rem;
    flex-shrink: 0;
  }

  .form-body {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .form-section {
    border: 1px solid var(--wc-border);
    padding: 16px;
    background: var(--wc-surface-alt);
    box-shadow: 2px 2px 0px var(--wc-shadow-solid, var(--wc-text));
  }

  .section-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.82rem;
    font-weight: 700;
    color: var(--wc-text);
    margin-bottom: 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .section-icon {
    font-size: 1.1rem;
    color: var(--wc-accent);
  }

  .row-fields {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .row-fields.split-2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .field-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .field-label {
    font-size: 0.78rem;
    font-weight: 700;
    color: var(--wc-text);
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  .nm-input {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    padding: 8px 12px !important;
    font-size: 0.92rem !important;
    box-shadow: none !important;
    width: 100%;
    box-sizing: border-box;
    height: auto !important;
  }

  .nm-input:focus {
    border-color: var(--wc-accent) !important;
    outline: none !important;
  }

  .nm-select {
    display: block;
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    padding: 8px 10px !important;
    font-size: 0.92rem !important;
    width: 100%;
    box-sizing: border-box;
    cursor: pointer;
  }

  .nm-select:focus {
    border-color: var(--wc-accent) !important;
    outline: none !important;
  }

  /* Players cards */
  .players-container {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .player-card {
    padding: 12px;
    border: 1px solid var(--wc-border-solid, var(--wc-text));
    background: var(--wc-surface);
  }

  .black-player-card {
    border-left: 4px solid var(--wc-go-black, #253530);
  }

  .white-player-card {
    border-left: 4px solid var(--wc-accent, #2e6b5e);
  }

  .player-header {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 8px;
  }

  .stone-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    display: inline-block;
  }

  .black-dot {
    background: var(--wc-go-black, #253530);
    border: 1px solid var(--wc-border-solid, var(--wc-text));
  }

  .white-dot {
    background: #ffffff;
    border: 1px solid var(--wc-border-solid, var(--wc-text));
  }

  .player-role {
    font-size: 0.78rem;
    font-weight: 700;
    color: var(--wc-text);
  }

  .player-inputs {
    display: flex;
    gap: 10px;
  }

  .player-inputs .name-field {
    flex: 7;
  }

  .player-inputs .rank-field {
    flex: 3;
  }

  /* Quick chips */
  .quick-chips {
    display: flex;
    gap: 6px;
    margin-top: 6px;
  }

  .chip-btn {
    border: 1px solid var(--wc-border-solid, var(--wc-text));
    background: var(--wc-surface);
    color: var(--wc-text);
    font-size: 0.72rem;
    font-family: 'JetBrains Mono', monospace;
    font-weight: 600;
    padding: 2px 8px;
    cursor: pointer;
    border-radius: 0;
    transition: var(--wc-transition-fast);
  }

  .chip-btn:hover {
    background: var(--wc-surface-alt);
  }

  .chip-btn.active {
    background: var(--wc-text);
    color: #ffffff;
  }

  /* Result pills */
  .result-pills {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .pill-btn {
    border: 1px solid var(--wc-border-solid, var(--wc-text));
    background: var(--wc-surface);
    color: var(--wc-text);
    font-size: 0.75rem;
    padding: 4px 8px;
    cursor: pointer;
    border-radius: 0;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
    font-weight: 600;
    transition: var(--wc-transition-fast);
  }

  .pill-btn:hover {
    background: var(--wc-surface-alt);
  }

  .pill-btn.selected {
    background: var(--wc-accent);
    color: #ffffff;
    border-color: var(--wc-border-solid, var(--wc-text));
    box-shadow: 1px 1px 0px var(--wc-shadow-solid, var(--wc-text));
  }

  /* Footer */
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    border-top: 1.5px solid var(--wc-border);
    margin-top: 20px;
    padding-top: 16px;
  }

  .save-btn {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important;
    background: var(--wc-accent) !important;
    color: #FFFFFF !important;
    box-shadow: 3px 3px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
    padding: 8px 24px !important;
    font-weight: 700;
    font-size: 0.88rem;
    font-family: 'DM Sans', sans-serif;
    cursor: pointer;
    transition: var(--wc-transition-fast);
  }

  .save-btn:hover:not(:disabled) {
    transform: translate(-1px, -1px);
    box-shadow: 4px 4px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
    background: var(--wc-accent-hover) !important;
  }

  .save-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .cancel-btn {
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    box-shadow: 3px 3px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
    padding: 8px 24px;
    font-weight: 700;
    font-size: 0.88rem;
    font-family: 'DM Sans', sans-serif;
    cursor: pointer;
    transition: var(--wc-transition-fast);
  }

  .cancel-btn:hover:not(:disabled) {
    transform: translate(-1px, -1px);
    box-shadow: 4px 4px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
    background: var(--wc-surface-alt) !important;
  }

  /* Fonts */
  .font-mincho {
    font-family: 'Shippori Mincho B1', serif;
  }

  .font-sans {
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  .font-mono {
    font-family: 'JetBrains Mono', monospace;
  }

  .animate-fade-in {
    animation: fadeIn 0.25s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @media only screen and (max-width: 500px) {
    .row-fields.split-2 {
      grid-template-columns: 1fr;
    }
    .modal-footer {
      flex-direction: column-reverse;
      gap: 10px;
    }
    .modal-footer button {
      width: 100% !important;
      margin: 0 !important;
    }
  }
</style>
