<script lang="ts">
  import { slide } from 'svelte/transition';
  import { createBoard, copyBoard, placeStone, coordsToSgf } from '../lib/goEngine';
  import Board from './Board.svelte';
  import CameraCaptureModal from './CameraCaptureModal.svelte';
  import { auth } from '../lib/auth.svelte';
  import { stringifySgf, SgfPlayer } from '../lib/sgfPlayer';
  import type { SgfNode } from '../lib/sgfPlayer';

  let { onSaveSuccess, onCancel } = $props<{
    onSaveSuccess: () => void;
    onCancel: () => void;
  }>();

  // Modal & Collapse States (Collapsed by default on mobile/tablet so board + controls fit cleanly)
  let showCameraModal = $state(false);
  let isMetadataCollapsed = $state(typeof window !== "undefined" ? window.innerWidth <= 992 : false);

  // Game Info Form States
  let gameTitle = $state("");
  let blackPlayer = $state("");
  let blackRank = $state("");
  let whitePlayer = $state("");
  let whiteRank = $state("");
  let gameDate = $state(new Date().toISOString().slice(0, 10));

  let metadataSummary = $derived.by(() => {
    const b = blackPlayer.trim() ? `${blackPlayer.trim()}${blackRank.trim() ? ' ' + blackRank.trim() : ''}` : '黒番';
    const w = whitePlayer.trim() ? `${whitePlayer.trim()}${whiteRank.trim() ? ' ' + whiteRank.trim() : ''}` : '白番';
    const titlePart = gameTitle.trim() ? `${gameTitle.trim()} — ` : '';
    return `${titlePart}● ${b} vs ○ ${w}`;
  });

  // Kifu creation history states
  interface CreatorMove {
    x: number | null;
    y: number | null;
    color: 1 | 2; // 1: Black, 2: White
    board: number[][];
    capturedCount: number;
  }

  let moves = $state<CreatorMove[]>([]);
  let currentMoveIndex = $state(-1);
  let size = $state(19);
  let saving = $state(false);

  // Derived states
  let currentBoard = $derived.by(() => {
    if (currentMoveIndex === -1) {
      return createBoard(size);
    }
    return moves[currentMoveIndex].board;
  });

  let lastMove = $derived.by(() => {
    if (currentMoveIndex === -1) return null;
    const m = moves[currentMoveIndex];
    if (m.x === null || m.y === null) return null;
    return { x: m.x, y: m.y };
  });

  let nextColor = $derived.by(() => {
    if (currentMoveIndex === -1) return 1; // Black starts
    return moves[currentMoveIndex].color === 1 ? 2 : 1;
  });

  let capturedByBlack = $derived.by(() => {
    return moves.slice(0, currentMoveIndex + 1)
      .filter(m => m.color === 1)
      .reduce((sum, m) => sum + m.capturedCount, 0);
  });

  let capturedByWhite = $derived.by(() => {
    return moves.slice(0, currentMoveIndex + 1)
      .filter(m => m.color === 2)
      .reduce((sum, m) => sum + m.capturedCount, 0);
  });

  const getM = () => (window as any).M;

  function handleIntersectionClick(detail: { x: number, y: number }) {
    const { x, y } = detail;

    // Check if move is valid using rules engine
    const res = placeStone(currentBoard, x, y, nextColor, size);
    if (!res.success || !res.board) {
      const M = getM();
      if (M) {
        M.toast({ html: `着手エラー: ${res.error || "無効な手です"}`, classes: 'red darken-2' });
      }
      return;
    }

    // If we were navigating back and placed a new stone, truncate future moves
    if (currentMoveIndex < moves.length - 1) {
      moves = moves.slice(0, currentMoveIndex + 1);
    }

    // Push new move
    moves.push({
      x,
      y,
      color: nextColor as 1 | 2,
      board: res.board,
      capturedCount: res.captured || 0
    });

    currentMoveIndex = moves.length - 1;
  }

  function handlePass() {
    const M = getM();
    // Cannot pass on empty board
    if (currentMoveIndex === -1 && moves.length === 0) {
      if (M) M.toast({ html: '開始前にパスはできません。', classes: 'orange' });
      return;
    }

    if (currentMoveIndex < moves.length - 1) {
      moves = moves.slice(0, currentMoveIndex + 1);
    }

    // A pass doesn't modify the board
    moves.push({
      x: null,
      y: null,
      color: nextColor as 1 | 2,
      board: currentBoard,
      capturedCount: 0
    });

    currentMoveIndex = moves.length - 1;

    if (M) {
      M.toast({ html: `${moves[currentMoveIndex].color === 1 ? '黒' : '白'}がパスしました。`, classes: 'grey darken-2' });
    }
  }

  function undo() {
    if (currentMoveIndex >= 0) {
      currentMoveIndex--;
    }
  }

  function redo() {
    if (currentMoveIndex < moves.length - 1) {
      currentMoveIndex++;
    }
  }

  function jumpToStart() {
    currentMoveIndex = -1;
  }

  function jumpToEnd() {
    currentMoveIndex = moves.length - 1;
  }

  // Keyboard navigation for move editing
  function handleKeyDown(e: KeyboardEvent) {
    const target = e.target as HTMLElement | null;
    const tag = (target?.tagName || '').toUpperCase();
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target?.isContentEditable) {
      return;
    }
    // Disable when camera modal is open
    if (showCameraModal) return;

    if (e.key === 'ArrowRight' || e.key === 'l' || e.key === 'L') {
      e.preventDefault();
      redo();
    } else if (e.key === 'ArrowLeft' || e.key === 'h' || e.key === 'H') {
      e.preventDefault();
      undo();
    } else if (e.key === ' ' || e.code === 'Space') {
      e.preventDefault();
      if (currentMoveIndex < moves.length - 1) {
        redo();
      } else if (moves.length > 0) {
        handlePass();
      }
    } else if (e.key === 'ArrowUp' || e.key === 'Home') {
      e.preventDefault();
      jumpToStart();
    } else if (e.key === 'ArrowDown' || e.key === 'End') {
      e.preventDefault();
      jumpToEnd();
    }
  }

  function handleImportFromCamera(detected: {
    board?: number[][];
    blackCount?: number;
    whiteCount?: number;
    sgf?: string;
  }) {
    const M = getM();

    // ⚡ 三脚リアルタイム対局レコーダーからの完全SGF手順取り込み
    if (detected.sgf) {
      try {
        const player = new SgfPlayer(detected.sgf);
        const newMoves: CreatorMove[] = [];

        for (let i = 1; i < player.history.length; i++) {
          const entry = player.history[i];
          if (!entry.node) continue;
          const isB = !!entry.node.properties.B;
          const isW = !!entry.node.properties.W;
          if (!isB && !isW) continue;

          newMoves.push({
            x: entry.lastMove ? entry.lastMove.x : null,
            y: entry.lastMove ? entry.lastMove.y : null,
            color: isB ? 1 : 2,
            board: copyBoard(entry.board),
            capturedCount: (isB ? entry.captives.W : entry.captives.B) || 0,
          });
        }

        if (newMoves.length > 0) {
          moves = newMoves;
          currentMoveIndex = newMoves.length - 1;

          if (M) {
            M.toast({
              html: `⚡ 三脚対局記録を取り込みました (${newMoves.length}手)`,
              classes: 'green darken-2'
            });
          }
          return;
        }
      } catch (err: any) {
        console.error("SGF import error from live recorder:", err);
      }
    }

    // 静止画盤面フォールバック (配置された石を一括復元)
    if (!detected.board) return;
    let curBoard = createBoard(size);
    const newMoves: CreatorMove[] = [];

    const blackStones: { x: number; y: number }[] = [];
    const whiteStones: { x: number; y: number }[] = [];

    for (let y = 0; y < 19; y++) {
      for (let x = 0; x < 19; x++) {
        if (detected.board[y][x] === 1) blackStones.push({ x, y });
        if (detected.board[y][x] === 2) whiteStones.push({ x, y });
      }
    }

    for (const b of blackStones) {
      curBoard = copyBoard(curBoard);
      curBoard[b.y][b.x] = 1;
      newMoves.push({
        x: b.x,
        y: b.y,
        color: 1,
        board: curBoard,
        capturedCount: 0,
      });
    }

    for (const w of whiteStones) {
      curBoard = copyBoard(curBoard);
      curBoard[w.y][w.x] = 2;
      newMoves.push({
        x: w.x,
        y: w.y,
        color: 2,
        board: curBoard,
        capturedCount: 0,
      });
    }

    moves = newMoves;
    currentMoveIndex = newMoves.length - 1;

    if (M) {
      M.toast({
        html: `📸 盤面を取り込みました (黒: ${detected.blackCount ?? blackStones.length}子, 白: ${detected.whiteCount ?? whiteStones.length}子)`,
        classes: 'green darken-2'
      });
    }
  }

  function generateSgf(): string {
    const ev = gameTitle.trim() || "ブラウザ対局";
    const pb = blackPlayer.trim() || "黒番";
    const pw = whitePlayer.trim() || "白番";
    const dt = gameDate.trim() || new Date().toISOString().slice(0,10);

    const properties: Record<string, string[]> = {
      GM: ["1"],
      FF: ["4"],
      SZ: [String(size)],
      PB: [pb],
      PW: [pw],
      DT: [dt],
      EV: [ev]
    };

    if (blackRank.trim()) properties.BR = [blackRank.trim()];
    if (whiteRank.trim()) properties.WR = [whiteRank.trim()];

    const rootNode: SgfNode = {
      properties,
      children: [],
      parent: null
    };

    let currentNode = rootNode;
    for (const m of moves) {
      const colorKey = m.color === 1 ? 'B' : 'W';
      const moveValue = (m.x === null || m.y === null) ? "" : coordsToSgf(m.x, m.y);
      
      const nextNode: SgfNode = {
        properties: {
          [colorKey]: [moveValue]
        },
        children: [],
        parent: currentNode
      };
      
      currentNode.children.push(nextNode);
      currentNode = nextNode;
    }

    return stringifySgf(rootNode);
  }

  async function handleSave() {
    if (moves.length === 0) {
      const M = getM();
      if (M) {
        M.toast({ html: '着手が1手も記録されていません。', classes: 'red' });
      }
      return;
    }

    saving = true;
    const sgfStr = generateSgf();
    const titleVal = gameTitle.trim() || `${blackPlayer || '黒'} vs ${whitePlayer || '白'}`;

    try {
      const payload = {
        title: titleVal,
        sgf_data: sgfStr
      };

      const res = await fetch('/api/kifus', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...auth.getHeaders()
        },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || "保存に失敗しました。");
      }

      const M = getM();
      if (M) {
        M.toast({ html: '棋譜がライブラリに保存されました！', classes: 'green' });
      }

      onSaveSuccess();
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: `保存エラー: ${err.message}`, classes: 'red' });
      }
    } finally {
      saving = false;
    }
  }
</script>
<svelte:window onkeydown={handleKeyDown} />

<div class="row animate-fade-in creator-container">
  <!-- Compact Top Header Bar -->
  <div class="col s12 creator-header">
    <h1 class="creator-title">
      <i class="material-icons" aria-hidden="true" style="font-size: 1.15rem; color: var(--wc-primary);">grid_on</i>
      <span>棋譜作成</span>
    </h1>
    <div class="creator-header-actions">
      <button type="button" class="nm-btn-flat creator-top-btn" onclick={() => showCameraModal = true} title="カメラや写真から盤面を取り込む">
        <i class="material-icons" aria-hidden="true">camera_enhance</i>
        <span>写真読込</span>
      </button>
      <button type="button" class="nm-btn-flat creator-top-btn" onclick={onCancel} disabled={saving}>
        <span>キャンセル</span>
      </button>
      <button type="button" class="nm-btn-primary creator-top-btn creator-save-btn" onclick={handleSave} disabled={moves.length === 0 || saving}>
        <i class="material-icons" aria-hidden="true">save</i>
        <span>{saving ? '保存中...' : '保存'}</span>
      </button>
    </div>
  </div>

  <!-- Left (Top on Mobile): Interactive Board -->
  <div class="col s12 l7 center-align kifu-board-column">
    <div class="board-wrapper">
      <Board 
        board={currentBoard} 
        {size} 
        {lastMove} 
        interactive={true} 
        turnColor={nextColor} 
        onIntersectionClick={handleIntersectionClick} 
      />
    </div>
  </div>

  <!-- Right (Bottom on Mobile): Controls FIRST, Metadata SECOND -->
  <div class="col s12 l5 creator-side-column">
    <!-- 1. Active Game Controls Card (Directly below board on mobile) -->
    <div class="em-portfolio-section controls-card">
      <!-- Compact Turn & Captives Bar -->
      <div class="compact-status-bar">
        <div class="turn-indicator">
          <span class="status-label">手番</span>
          <span
            class="stone-dot"
            style="background-color: {nextColor === 1 ? 'var(--wc-go-black)' : 'var(--wc-go-white)'};"
          ></span>
          <strong class="turn-text">{nextColor === 1 ? '黒番' : '白番'}</strong>
        </div>

        <div class="captives-inline">
          <span class="status-label">アゲハマ</span>
          <span class="captive-badge">
            <span class="stone-dot-sm" style="background-color: var(--wc-go-black);"></span>
            <strong>{capturedByBlack}</strong>
          </span>
          <span class="captive-badge">
            <span class="stone-dot-sm" style="background-color: var(--wc-go-white);"></span>
            <strong>{capturedByWhite}</strong>
          </span>
        </div>
      </div>

      <!-- Single-Row Navigation & Pass Controls -->
      <div class="control-buttons-row">
        <div class="control-buttons">
          <button type="button" class="nm-btn-flat" onclick={jumpToStart} disabled={currentMoveIndex === -1} title="最初へ" aria-label="最初の手へ移動">
            <i class="material-icons" aria-hidden="true">first_page</i>
          </button>
          <button type="button" class="nm-btn-flat" onclick={undo} disabled={currentMoveIndex === -1} title="1手戻る" aria-label="1手戻る">
            <i class="material-icons" aria-hidden="true">chevron_left</i>
          </button>
          <button type="button" class="nm-btn-flat" onclick={redo} disabled={currentMoveIndex === moves.length - 1} title="1手進む" aria-label="1手進む">
            <i class="material-icons" aria-hidden="true">chevron_right</i>
          </button>
          <button type="button" class="nm-btn-flat" onclick={jumpToEnd} disabled={currentMoveIndex === moves.length - 1} title="最後へ" aria-label="最後の手へ移動">
            <i class="material-icons" aria-hidden="true">last_page</i>
          </button>
        </div>

        <button
          type="button"
          class="nm-btn pass-inline-btn"
          onclick={handlePass}
          disabled={saving}
          title="パスする"
        >
          <i class="material-icons" aria-hidden="true">redo</i>
          <span>パス</span>
        </button>
      </div>
    </div>

    <!-- 2. Game Metadata Card (Collapsible, placed below controls) -->
    <div
      class="em-portfolio-section metadata-card"
      class:is-collapsed={isMetadataCollapsed}
      style="border-color: var(--wc-border-solid, var(--wc-text)) !important;"
    >
      <button
        type="button"
        class="metadata-toggle-btn"
        aria-expanded={!isMetadataCollapsed}
        aria-controls="creator-metadata-panel"
        onclick={() => (isMetadataCollapsed = !isMetadataCollapsed)}
        title={isMetadataCollapsed ? "対局情報の入力欄を展開する" : "対局情報の入力欄を折りたたむ"}
      >
        <div class="metadata-toggle-left">
          <i class="material-icons" aria-hidden="true" style="font-size: 1.1rem; color: var(--wc-primary);">assignment</i>
          <span class="metadata-toggle-title">対局情報</span>
          {#if isMetadataCollapsed}
            <span class="metadata-summary-chip">{metadataSummary}</span>
          {/if}
        </div>
        <span class="metadata-toggle-action">
          <span>{isMetadataCollapsed ? "ひらく" : "たたむ"}</span>
          <i
            class="material-icons metadata-chevron"
            class:is-collapsed={isMetadataCollapsed}
            aria-hidden="true"
          >
            expand_less
          </i>
        </span>
      </button>

      {#if !isMetadataCollapsed}
        <div id="creator-metadata-panel" class="card-content" transition:slide={{ duration: 180 }} style="padding: 14px 0 0 0;">
          <div class="row" style="margin-bottom: 0;">
            <div class="input-field col s12" style="margin-top: 0; margin-bottom: 12px;">
              <input id="game_title" type="text" class="nm-input" bind:value={gameTitle} placeholder="例: 第1期 〇〇戦" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important; background: var(--wc-surface) !important;" />
              <label for="game_title" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">対局名 / タイトル</label>
            </div>

            <div class="input-field col s8" style="margin-top: 5px; margin-bottom: 12px;">
              <input id="black_player" type="text" class="nm-input" bind:value={blackPlayer} placeholder="黒番プレイヤー" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important; background: var(--wc-surface) !important;" />
              <label for="black_player" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">黒番 (PB)</label>
            </div>
            <div class="input-field col s4" style="margin-top: 5px; margin-bottom: 12px;">
              <input id="black_rank" type="text" class="nm-input" bind:value={blackRank} placeholder="例: 9段" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important; background: var(--wc-surface) !important;" />
              <label for="black_rank" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">段位</label>
            </div>

            <div class="input-field col s8" style="margin-top: 5px; margin-bottom: 12px;">
              <input id="white_player" type="text" class="nm-input" bind:value={whitePlayer} placeholder="白番プレイヤー" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important; background: var(--wc-surface) !important;" />
              <label for="white_player" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">白番 (PW)</label>
            </div>
            <div class="input-field col s4" style="margin-top: 5px; margin-bottom: 12px;">
              <input id="white_rank" type="text" class="nm-input" bind:value={whiteRank} placeholder="例: 8段" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important; background: var(--wc-surface) !important;" />
              <label for="white_rank" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">段位</label>
            </div>

            <div class="input-field col s12" style="margin-top: 5px; margin-bottom: 5px;">
              <input id="game_date" type="date" class="nm-input" bind:value={gameDate} style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important; background: var(--wc-surface) !important;" />
              <label for="game_date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">対局日 (DT)</label>
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>
</div>

<CameraCaptureModal
  isOpen={showCameraModal}
  onClose={() => showCameraModal = false}
  onImport={handleImportFromCamera}
/>

<style>
  .animate-fade-in {
    animation: fadeIn 0.25s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(10px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .creator-container {
    margin-top: 0.5rem;
    margin-bottom: 0.5rem;
  }

  /* Compact Top Header Bar */
  .creator-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    margin-bottom: 0.65rem;
    padding-bottom: 8px;
    border-bottom: 1.5px solid var(--wc-border-solid, var(--wc-text));
  }

  .creator-title {
    margin: 0;
    font-size: 1.05rem;
    font-family: 'Shippori Mincho B1', serif;
    font-weight: 700;
    color: var(--wc-text);
    display: inline-flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
  }

  .creator-header-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: nowrap;
  }

  .creator-top-btn {
    height: 32px !important;
    line-height: 30px !important;
    padding: 0 10px !important;
    font-size: 0.78rem !important;
    font-weight: 700 !important;
    border-radius: 0 !important;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important;
    background: var(--wc-surface-alt) !important;
    color: var(--wc-text) !important;
    display: inline-flex !important;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
  }

  .creator-top-btn i {
    font-size: 1rem !important;
    line-height: 1 !important;
  }

  .creator-save-btn {
    background: var(--wc-primary) !important;
    color: #fff !important;
    box-shadow: 2px 2px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
  }

  /* Layout alignment and sizing */
  .kifu-board-column {
    display: flex;
    flex-direction: column;
    align-items: center;
    margin-bottom: 1rem;
  }

  .board-wrapper {
    border: 2px solid transparent;
    border-radius: 8px;
    padding: 2px;
    transition: all 0.25s cubic-bezier(0.25, 0.8, 0.25, 1);
    display: inline-block;
    width: 100%;
    max-width: min(72vh, 680px);
    box-sizing: border-box;
  }

  /* Compact Controls Card (Placed directly below board on mobile) */
  .controls-card {
    margin-bottom: 0.85rem;
    padding: 12px 14px !important;
    border-color: var(--wc-border-solid, var(--wc-text)) !important;
  }

  .compact-status-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 10px;
    margin-bottom: 10px;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text));
    background: var(--wc-surface-alt);
    box-shadow: 2px 2px 0px var(--wc-shadow-solid, var(--wc-text));
  }

  .turn-indicator {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .status-label {
    font-size: 0.72rem;
    color: var(--wc-text-muted);
    font-weight: 700;
  }

  .stone-dot {
    display: inline-block;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text));
  }

  .turn-text {
    font-size: 0.85rem;
    color: var(--wc-text);
  }

  .captives-inline {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    font-size: 0.8rem;
    color: var(--wc-text);
  }

  .captive-badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.85rem;
  }

  .stone-dot-sm {
    display: inline-block;
    width: 9px;
    height: 9px;
    border-radius: 50%;
    border: 1px solid var(--wc-border-solid, var(--wc-text));
  }

  .control-buttons-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .control-buttons {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .control-buttons button {
    width: 40px;
    height: 38px;
    min-width: 40px;
    padding: 0 !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important;
    box-shadow: 2px 2px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
    border-radius: 0px !important;
    display: inline-flex !important;
    align-items: center;
    justify-content: center;
  }

  .control-buttons button i {
    font-size: 1.35rem;
    color: var(--wc-text);
  }

  .pass-inline-btn {
    height: 38px !important;
    line-height: 36px !important;
    padding: 0 14px !important;
    border-radius: 0 !important;
    border: 1.5px solid var(--wc-border-solid, var(--wc-text)) !important;
    background: var(--wc-surface) !important;
    box-shadow: 2px 2px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
    color: var(--wc-text) !important;
    font-weight: 700 !important;
    font-size: 0.82rem !important;
    display: inline-flex !important;
    align-items: center;
    justify-content: center;
    gap: 4px;
    white-space: nowrap;
  }

  .pass-inline-btn i {
    font-size: 1.1rem !important;
  }

  /* Collapsible Metadata Card (Washi Clay / Vogue Neo-Brutalist) */
  .metadata-card {
    margin-bottom: 1.2rem;
    padding: 14px 14px 12px 14px !important;
    transition: padding 0.2s ease, margin-bottom 0.2s ease;
  }

  .metadata-card.is-collapsed {
    margin-bottom: 1rem;
    padding: 10px 12px !important;
  }

  .metadata-toggle-btn {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 10px;
    background: var(--wc-surface-alt);
    border: 1.5px solid var(--wc-border-solid, var(--wc-text));
    box-shadow: 2px 2px 0px var(--wc-shadow-solid, var(--wc-text));
    border-radius: 0 !important;
    color: var(--wc-text);
    cursor: pointer;
    text-align: left;
    transition: transform 0.12s ease, box-shadow 0.12s ease, background 0.12s ease;
  }

  .metadata-toggle-btn:hover {
    background: var(--wc-surface);
    transform: translate(-1px, -1px);
    box-shadow: 3px 3px 0px var(--wc-shadow-solid, var(--wc-text));
  }

  .metadata-toggle-btn:active {
    transform: translate(1px, 1px);
    box-shadow: 1px 1px 0px var(--wc-shadow-solid, var(--wc-text));
  }

  .metadata-toggle-left {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex: 1;
  }

  .metadata-toggle-title {
    font-size: 0.85rem;
    font-weight: 700;
    letter-spacing: 0.03em;
    white-space: nowrap;
  }

  .metadata-summary-chip {
    font-size: 0.74rem;
    font-weight: 600;
    color: var(--wc-text-muted);
    background: var(--wc-surface);
    border: 1px solid var(--wc-border-solid, var(--wc-text));
    padding: 1px 8px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 100%;
  }

  .metadata-toggle-action {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    font-size: 0.74rem;
    font-weight: 700;
    color: var(--wc-text);
    white-space: nowrap;
    flex-shrink: 0;
  }

  .metadata-chevron {
    font-size: 1.2rem;
    transition: transform 0.2s ease;
  }

  .metadata-chevron.is-collapsed {
    transform: rotate(180deg);
  }

  @media (prefers-reduced-motion: reduce) {
    .metadata-card,
    .metadata-toggle-btn,
    .metadata-chevron {
      transition: none !important;
    }
  }

  /* Touch and Tablet responsive adjustments */
  @media (pointer: coarse), only screen and (max-width: 992px) {
    .kifu-board-column {
      margin-bottom: 0.45rem !important;
    }
    .board-wrapper {
      max-width: min(100%, 54dvh) !important;
    }
    .control-buttons button {
      width: 44px !important;
      height: 40px !important;
      min-width: 44px !important;
    }
    .control-buttons button i {
      font-size: 1.5rem !important;
    }
    .pass-inline-btn {
      height: 40px !important;
      line-height: 38px !important;
    }
  }

  /* Mobile screen responsive adjustments (狭い画面で碁盤とナビが1画面に収まる最適化) */
  @media only screen and (max-width: 600px) {
    .creator-container {
      margin-top: 0.2rem !important;
    }
    .creator-header {
      margin-bottom: 0.35rem !important;
      padding-bottom: 5px !important;
    }
    .creator-title {
      font-size: 0.92rem !important;
    }
    .creator-top-btn {
      height: 28px !important;
      line-height: 26px !important;
      padding: 0 7px !important;
      font-size: 0.72rem !important;
    }
    .kifu-board-column {
      padding-left: 2px !important;
      padding-right: 2px !important;
      margin-bottom: 0.3rem !important;
    }
    .board-wrapper {
      border-width: 1px !important;
      padding: 0 !important;
      border-radius: 4px !important;
      max-width: min(100%, 52dvh) !important;
    }
    .creator-side-column {
      padding-left: 4px !important;
      padding-right: 4px !important;
    }
    .controls-card {
      padding: 8px 10px !important;
      margin-bottom: 0.5rem !important;
    }
    .compact-status-bar {
      padding: 4px 8px !important;
      margin-bottom: 6px !important;
    }
  }
</style>
