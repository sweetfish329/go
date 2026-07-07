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
  <div class="col s12 d-flex align-center justify-between" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; flex-wrap: wrap; gap: 12px;">
    <h4 style="margin: 0; font-weight: 600; font-family: 'Shippori Mincho B1', serif; color: var(--wc-text); letter-spacing: 0.02em;">新規棋譜の作成</h4>
    <div style="display: flex; gap: 10px;">
      <button class="nm-btn-flat" onclick={onCancel} disabled={saving}>キャンセル</button>
      <button class="nm-btn-primary" onclick={handleSave} disabled={moves.length === 0 || saving}>
        <i class="material-icons" style="font-size: 1.15rem;">save</i>{saving ? '保存中...' : 'ライブラリに保存'}
      </button>
    </div>
  </div>

  <!-- Left: Interactive Board -->
  <div class="col s12 l7 center-align" style="margin-bottom: 1.5rem;">
    <div style="display: inline-block;">
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
    <div class="nm-card" style="margin-bottom: 1.5rem;">
      <div class="card-content" style="padding: 20px;">
        <span class="card-title" style="font-size: 1.1rem; font-weight: 600; margin-bottom: 18px; color: var(--wc-accent); font-family: 'Shippori Mincho B1', serif;">対局情報の入力</span>
        
        <div class="row" style="margin-bottom: 0;">
          <div class="input-field col s12" style="margin-top: 0; margin-bottom: 10px;">
            <input id="game_title" type="text" class="nm-input" bind:value={gameTitle} placeholder="例: 第1期 〇〇戦" style="margin-bottom: 0;" />
            <label for="game_title" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">対局名 / タイトル</label>
          </div>

          <div class="input-field col s8" style="margin-top: 5px; margin-bottom: 10px;">
            <input id="black_player" type="text" class="nm-input" bind:value={blackPlayer} placeholder="黒番プレイヤー" style="margin-bottom: 0;" />
            <label for="black_player" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">黒番 (PB)</label>
          </div>
          <div class="input-field col s4" style="margin-top: 5px; margin-bottom: 10px;">
            <input id="black_rank" type="text" class="nm-input" bind:value={blackRank} placeholder="例: 9段" style="margin-bottom: 0;" />
            <label for="black_rank" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">段位</label>
          </div>

          <div class="input-field col s8" style="margin-top: 5px; margin-bottom: 10px;">
            <input id="white_player" type="text" class="nm-input" bind:value={whitePlayer} placeholder="白番プレイヤー" style="margin-bottom: 0;" />
            <label for="white_player" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">白番 (PW)</label>
          </div>
          <div class="input-field col s4" style="margin-top: 5px; margin-bottom: 10px;">
            <input id="white_rank" type="text" class="nm-input" bind:value={whiteRank} placeholder="例: 8段" style="margin-bottom: 0;" />
            <label for="white_rank" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">段位</label>
          </div>

          <div class="input-field col s12" style="margin-top: 5px; margin-bottom: 5px;">
            <input id="game_date" type="date" class="nm-input" bind:value={gameDate} style="margin-bottom: 0;" />
            <label for="game_date" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text-muted);">対局日 (DT)</label>
          </div>
        </div>
      </div>
    </div>

    <!-- Active Game State & Controls Card -->
    <div class="nm-card" style="margin-bottom: 1.5rem;">
      <div class="card-content" style="padding: 20px;">
        <span class="card-title" style="font-size: 1.1rem; font-weight: 600; margin-bottom: 18px; color: var(--wc-accent); font-family: 'Shippori Mincho B1', serif;">対局状況 & 操作</span>

        <!-- Hand & Captives Stats -->
        <div class="stats-panel nm-panel-inset" style="display: flex; justify-content: space-around; padding: 12px; margin-bottom: 15px;">
          <div class="stat-item center-align">
            <div style="font-size: 0.8rem; color: var(--wc-text-muted);">現在の手数</div>
            <div style="font-size: 1.25rem; font-weight: 600; color: var(--wc-text); font-family: 'JetBrains Mono', monospace; margin-top: 2px;">{currentMoveIndex + 1} 手目</div>
          </div>
          <div class="stat-item center-align">
            <div style="font-size: 0.8rem; color: var(--wc-text-muted);">次の一手</div>
            <div style="display: flex; align-items: center; justify-content: center; gap: 6px; font-weight: 500; margin-top: 4px;">
              <span class="stone-dot" style="display: inline-block; width: 12px; height: 12px; border-radius: 50%; background-color: {nextColor === 1 ? 'var(--wc-go-black)' : 'var(--wc-go-white)'}; border: 1px solid {nextColor === 1 ? 'var(--wc-go-black)' : 'var(--wc-border)'};"></span>
              <span style="font-size: 0.9rem; color: var(--wc-text);">{nextColor === 1 ? '黒番' : '白番'}</span>
            </div>
          </div>
        </div>

        <div class="captives-panel nm-panel-inset" style="display: flex; justify-content: space-around; padding: 10px; margin-bottom: 20px; font-size: 0.85rem; color: var(--wc-text);">
          <div style="display: flex; align-items: center; gap: 6px;">
            <span style="display: inline-block; width: 8px; height: 8px; border-radius: 50%; background-color: var(--wc-go-black);"></span>
            <span>黒のアゲハマ: <strong style="font-family: 'JetBrains Mono', monospace; font-size: 0.95rem;">{capturedByBlack}</strong></span>
          </div>
          <div style="display: flex; align-items: center; gap: 6px;">
            <span style="display: inline-block; width: 8px; height: 8px; border-radius: 50%; background-color: var(--wc-go-white); border: 1px solid var(--wc-border);"></span>
            <span>白のアゲハマ: <strong style="font-family: 'JetBrains Mono', monospace; font-size: 0.95rem;">{capturedByWhite}</strong></span>
          </div>
        </div>

        <!-- Navigation Buttons -->
        <div class="control-buttons center-align" style="margin-bottom: 15px; display: flex; justify-content: center; gap: 12px;">
          <button class="nm-btn-flat nm-btn-round" onclick={jumpToStart} disabled={currentMoveIndex === -1} title="最初へ">
            <i class="material-icons">first_page</i>
          </button>
          <button class="nm-btn-flat nm-btn-round" onclick={undo} disabled={currentMoveIndex === -1} title="戻る">
            <i class="material-icons">chevron_left</i>
          </button>
          <button class="nm-btn-flat nm-btn-round" onclick={redo} disabled={currentMoveIndex === moves.length - 1} title="進む">
            <i class="material-icons">chevron_right</i>
          </button>
          <button class="nm-btn-flat nm-btn-round" onclick={jumpToEnd} disabled={currentMoveIndex === moves.length - 1} title="最後へ">
            <i class="material-icons">last_page</i>
          </button>
        </div>

        <!-- Action Buttons -->
        <div style="display: flex; flex-direction: column; gap: 10px;">
          <button class="nm-btn" onclick={handlePass} disabled={saving} style="width: 100%;">
            <i class="material-icons" style="font-size: 1.15rem;">redo</i>パスする
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
</style>
