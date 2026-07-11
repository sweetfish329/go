<script lang="ts">
  import { createBoard, placeStone, coordsToSgf } from '../lib/goEngine';
  import Board from './Board.svelte';
  import { auth } from '../lib/auth.svelte';
  import { stringifySgf } from '../lib/sgfPlayer';
  import type { SgfNode } from '../lib/sgfPlayer';

  let { onSaveSuccess, onCancel } = $props<{
    onSaveSuccess: () => void;
    onCancel: () => void;
  }>();

  // Game Info Form States
  let gameTitle = $state("");
  let blackPlayer = $state("");
  let blackRank = $state("");
  let whitePlayer = $state("");
  let whiteRank = $state("");
  let gameDate = $state(new Date().toISOString().slice(0, 10));

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

  function handleIntersectionClick(e: CustomEvent<{ x: number, y: number }>) {
    const { x, y } = e.detail;

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
      const res = await fetch('/api/kifus', {
        method: 'POST',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          title: titleVal,
          sgf_data: sgfStr
        })
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

<div class="row animate-fade-in" style="margin-top: 1.5rem;">
  <div class="col s12 d-flex align-center justify-between" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 2rem; flex-wrap: wrap; gap: 12px; border-bottom: 2px solid var(--wc-text); padding-bottom: 16px; position: relative; z-index: 10;">
    <div style="position: relative;">
      <!-- Overlap decorative text -->
      <span class="em-collage-tag-pastel em-float-badge" style="font-size: 0.62rem; position: absolute; top: -16px; left: 0; box-shadow: 2px 2px 0px var(--wc-text);">
        CREATOR STUDIO
      </span>
      <h1 class="em-newspaper-headline" style="margin: 6px 0 0 0; font-size: 1.6rem; font-family: 'Shippori Mincho B1', serif; font-weight: 700; color: var(--wc-text);">新規棋譜の作成 — Record Creator</h1>
    </div>
    <div style="display: flex; gap: 10px;">
      <button class="nm-btn-flat" style="border-radius: 0 !important; font-weight: bold;" onclick={onCancel} disabled={saving}>キャンセル</button>
      <button class="nm-btn-primary" style="border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; box-shadow: 3px 3px 0px var(--wc-text) !important; font-weight: bold;" onclick={handleSave} disabled={moves.length === 0 || saving}>
        <i class="material-icons" style="font-size: 1.15rem; vertical-align: middle; margin-right: 4px;">save</i>{saving ? '保存中...' : 'ライブラリに保存'}
      </button>
    </div>
  </div>

  <!-- Left: Interactive Board -->
  <div class="col s12 l7 center-align kifu-board-column" style="margin-bottom: 2rem;">
    <div class="board-wrapper" style="display: inline-block; position: relative;">
      <Board 
        board={currentBoard} 
        {size} 
        {lastMove} 
        interactive={true} 
        turnColor={nextColor} 
        on:intersectionClick={handleIntersectionClick} 
      />
    </div>
  </div>

  <!-- Right: Control Panel & Metadata Form -->
  <div class="col s12 l5">
    <!-- Game Metadata Card -->
    <div class="em-portfolio-section" style="margin-bottom: 2rem; border-color: var(--wc-text) !important; padding: 28px 20px 20px 20px !important;">
      <!-- Overlapping Badge -->
      <span class="em-collage-tag" style="position: absolute; top: -14px; left: 16px; z-index: 10; font-size: 0.72rem;">
        Specs — Metadata Input
      </span>

      <div class="card-content" style="padding: 12px 0 0 0;">
        <div class="row" style="margin-bottom: 0;">
          <div class="input-field col s12" style="margin-top: 0; margin-bottom: 12px;">
            <input id="game_title" type="text" class="nm-input" bind:value={gameTitle} placeholder="例: 第1期 〇〇戦" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
            <label for="game_title" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">対局名 / タイトル</label>
          </div>

          <div class="input-field col s8" style="margin-top: 5px; margin-bottom: 12px;">
            <input id="black_player" type="text" class="nm-input" bind:value={blackPlayer} placeholder="黒番プレイヤー" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
            <label for="black_player" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">黒番 (PB)</label>
          </div>
          <div class="input-field col s4" style="margin-top: 5px; margin-bottom: 12px;">
            <input id="black_rank" type="text" class="nm-input" bind:value={blackRank} placeholder="例: 9段" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
            <label for="black_rank" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">段位</label>
          </div>

          <div class="input-field col s8" style="margin-top: 5px; margin-bottom: 12px;">
            <input id="white_player" type="text" class="nm-input" bind:value={whitePlayer} placeholder="白番プレイヤー" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
            <label for="white_player" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">白番 (PW)</label>
          </div>
          <div class="input-field col s4" style="margin-top: 5px; margin-bottom: 12px;">
            <input id="white_rank" type="text" class="nm-input" bind:value={whiteRank} placeholder="例: 8段" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
            <label for="white_rank" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">段位</label>
          </div>

          <div class="input-field col s12" style="margin-top: 5px; margin-bottom: 5px;">
            <input id="game_date" type="date" class="nm-input" bind:value={gameDate} style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
            <label for="game_date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">対局日 (DT)</label>
          </div>
        </div>
      </div>
    </div>

    <!-- Active Game State & Controls Card -->
    <div class="em-portfolio-section" style="margin-bottom: 1.5rem; border-color: var(--wc-text) !important; padding: 28px 20px 20px 20px !important;">
      <!-- Overlapping Badge -->
      <span class="em-collage-tag" style="position: absolute; top: -14px; left: 16px; z-index: 10; font-size: 0.72rem; transform: rotate(1deg);">
        Engine & Navigation — Controls
      </span>

      <div class="card-content" style="padding: 12px 0 0 0;">
        <!-- Hand & Captives Stats -->
        <div class="stats-panel" style="display: flex; justify-content: space-around; padding: 12px; margin-bottom: 15px; border: 1.5px solid var(--wc-text); background: var(--wc-surface-alt); box-shadow: 2px 2px 0px var(--wc-text); transform: rotate(-0.5deg);">
          <div class="stat-item center-align">
            <div style="font-size: 0.75rem; color: var(--wc-text-muted); font-weight: bold; letter-spacing: 0.05em;">現在の手数</div>
            <div style="font-size: 1.2rem; font-weight: 700; color: var(--wc-text); font-family: 'JetBrains Mono', monospace; margin-top: 2px;">{currentMoveIndex + 1} 手目</div>
          </div>
          <div class="stat-item center-align">
            <div style="font-size: 0.75rem; color: var(--wc-text-muted); font-weight: bold; letter-spacing: 0.05em;">次の一手</div>
            <div style="display: flex; align-items: center; justify-content: center; gap: 6px; font-weight: 700; margin-top: 4px;">
              <span class="stone-dot" style="display: inline-block; width: 12px; height: 12px; border-radius: 50%; background-color: {nextColor === 1 ? 'var(--wc-go-black)' : 'var(--wc-go-white)'}; border: 1.5px solid var(--wc-text);"></span>
              <span style="font-size: 0.88rem; color: var(--wc-text);">{nextColor === 1 ? '黒番' : '白番'}</span>
            </div>
          </div>
        </div>

        <div class="captives-panel" style="display: flex; justify-content: space-around; padding: 10px; margin-bottom: 20px; font-size: 0.8rem; color: var(--wc-text); border: 1.5px solid var(--wc-text); background: var(--wc-surface-alt); box-shadow: 2px 2px 0px var(--wc-text); transform: rotate(0.5deg);">
          <div style="display: flex; align-items: center; gap: 6px; font-weight: 600;">
            <span style="display: inline-block; width: 8px; height: 8px; border-radius: 50%; background-color: var(--wc-go-black); border: 1px solid var(--wc-text);"></span>
            <span>黒のアゲハマ: <strong style="font-family: 'JetBrains Mono', monospace; font-size: 0.9rem; color: var(--wc-text);">{capturedByBlack}</strong></span>
          </div>
          <div style="display: flex; align-items: center; gap: 6px; font-weight: 600;">
            <span style="display: inline-block; width: 8px; height: 8px; border-radius: 50%; background-color: var(--wc-go-white); border: 1px solid var(--wc-text);"></span>
            <span>白のアゲハマ: <strong style="font-family: 'JetBrains Mono', monospace; font-size: 0.9rem; color: var(--wc-text);">{capturedByWhite}</strong></span>
          </div>
        </div>

        <!-- Navigation Buttons -->
        <div class="control-buttons center-align" style="margin-bottom: 15px; display: flex; justify-content: center; gap: 8px;">
          <button class="nm-btn-flat" onclick={jumpToStart} disabled={currentMoveIndex === -1} title="最初へ">
            <i class="material-icons" style="font-size: 1.25rem; color: var(--wc-text);">first_page</i>
          </button>
          <button class="nm-btn-flat" onclick={undo} disabled={currentMoveIndex === -1} title="戻る">
            <i class="material-icons" style="font-size: 1.25rem; color: var(--wc-text);">chevron_left</i>
          </button>
          <button class="nm-btn-flat" onclick={redo} disabled={currentMoveIndex === moves.length - 1} title="進む">
            <i class="material-icons" style="font-size: 1.25rem; color: var(--wc-text);">chevron_right</i>
          </button>
          <button class="nm-btn-flat" onclick={jumpToEnd} disabled={currentMoveIndex === moves.length - 1} title="最後へ">
            <i class="material-icons" style="font-size: 1.25rem; color: var(--wc-text);">last_page</i>
          </button>
        </div>

        <!-- Action Buttons -->
        <div style="display: flex; flex-direction: column; gap: 10px;">
          <button class="nm-btn em-pulse-button" style="width: 100%; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important; box-shadow: 3px 3px 0px var(--wc-text) !important; color: var(--wc-text) !important; font-weight: bold;" onclick={handlePass} disabled={saving}>
            <i class="material-icons" style="font-size: 1.15rem; vertical-align: middle; margin-right: 4px;">redo</i>パスする (Pass)
          </button>
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  .animate-fade-in {
    animation: fadeIn 0.3s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(15px); }
    to { opacity: 1; transform: translateY(0); }
  }

  /* Layout alignment and sizing */
  .kifu-board-column {
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .board-wrapper {
    border: 4px solid transparent;
    border-radius: 12px;
    padding: 4px;
    transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
    display: inline-block;
    width: 100%;
    max-width: min(78vh, 720px);
    box-sizing: border-box;
  }

  .control-buttons button {
    width: 38px;
    height: 38px;
    min-width: 38px;
    padding: 0 !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    border: 1.5px solid var(--wc-text) !important;
    box-shadow: 2px 2px 0px var(--wc-text) !important;
    border-radius: 0px !important;
    display: inline-flex !important;
    align-items: center;
    justify-content: center;
  }

  /* Mobile responsive adjustments */
  @media only screen and (max-width: 600px) {
    .kifu-board-column {
      padding-left: 6px !important;
      padding-right: 6px !important;
    }
    .board-wrapper {
      border-width: 2px !important;
      padding: 2px !important;
      border-radius: 6px !important;
    }
    .control-buttons {
      gap: 12px !important;
    }
    .control-buttons button {
      width: 46px !important;
      height: 46px !important;
      line-height: 46px !important;
      min-width: 46px !important;
    }
    .control-buttons button i {
      font-size: 1.6rem !important;
      line-height: 46px !important;
    }
  }
</style>
