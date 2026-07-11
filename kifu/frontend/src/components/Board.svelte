<script lang="ts">
  let {
    board = [],
    size = 19,
    lastMove = null,
    interactive = true,
    turnColor = 1,
    onIntersectionClick
  } = $props<{
    board?: number[][];
    size?: number;
    lastMove?: { x: number; y: number } | null;
    interactive?: boolean;
    turnColor?: number;
    onIntersectionClick?: (detail: { x: number; y: number }) => void;
  }>();

  let svgElement = $state<SVGSVGElement>();
  let hoverIntersection = $state<{ x: number; y: number } | null>(null); // { x, y } under mouse pointer
  const isMobileDevice = $derived(typeof window !== 'undefined' && 
    (/Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent) ||
     window.innerWidth < 768 || 
     window.matchMedia('(pointer: coarse)').matches));

  const padding = 25;
  const boardSize = 500;
  const usableSize = boardSize - padding * 2;
  const step = $derived(usableSize / (size - 1));

  interface StarPoint {
    x: number;
    y: number;
  }

  // Generate star point coordinates
  const starPoints = $derived(getStarPoints(size));

  function getStarPoints(s: number): StarPoint[] {
    if (s === 19) {
      const idxs = [3, 9, 15];
      const pts: StarPoint[] = [];
      for (const x of idxs) {
        for (const y of idxs) {
          pts.push({ x, y });
        }
      }
      return pts;
    } else if (s === 13) {
      return [
        { x: 3, y: 3 }, { x: 3, y: 9 },
        { x: 9, y: 3 }, { x: 9, y: 9 },
        { x: 6, y: 6 }
      ];
    } else if (s === 9) {
      return [
        { x: 2, y: 2 }, { x: 2, y: 6 },
        { x: 6, y: 2 }, { x: 6, y: 6 },
        { x: 4, y: 4 }
      ];
    }
    return [];
  }

  // Map coordinates to SVG pixels
  function getPos(index: number): number {
    return padding + index * step;
  }

  // Calculate intersection from click
  function handleSvgClick(event: MouseEvent): void {
    if (!interactive || !svgElement) return;

    const rect = svgElement.getBoundingClientRect();
    const clientX = event.clientX - rect.left;
    const clientY = event.clientY - rect.top;

    const svgX = (clientX / rect.width) * boardSize;
    const svgY = (clientY / rect.height) * boardSize;

    const x = Math.round((svgX - padding) / step);
    const y = Math.round((svgY - padding) / step);

    if (x >= 0 && x < size && y >= 0 && y < size) {
      onIntersectionClick?.({ x, y });
    }
  }

  // Calculate coordinates on mouse move for hover effect
  function handleMouseMove(event: MouseEvent): void {
    if (!interactive || !svgElement || isMobileDevice) return;

    const rect = svgElement.getBoundingClientRect();
    const clientX = event.clientX - rect.left;
    const clientY = event.clientY - rect.top;

    const svgX = (clientX / rect.width) * boardSize;
    const svgY = (clientY / rect.height) * boardSize;

    const x = Math.round((svgX - padding) / step);
    const y = Math.round((svgY - padding) / step);

    if (x >= 0 && x < size && y >= 0 && y < size && board[y]?.[x] === 0) {
      hoverIntersection = { x, y };
    } else {
      hoverIntersection = null;
    }
  }

  function handleMouseLeave(): void {
    hoverIntersection = null;
  }
</script>

<div class="board-container">
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <svg
    bind:this={svgElement}
    viewBox="0 0 {boardSize} {boardSize}"
    class="go-board"
    onclick={handleSvgClick}
    onmousemove={handleMouseMove}
    onmouseleave={handleMouseLeave}
  >
    <!-- Board background (Nordic pastel flat style) -->
    <rect width={boardSize} height={boardSize} fill="var(--wc-board)" />
    <rect width={boardSize} height={boardSize} fill="none" stroke="var(--wc-text)" stroke-width="3" />

    <!-- Grid lines -->
    <!-- Horizontal lines -->
    {#each Array(size) as _, i (i)}
      <line
        x1={padding}
        y1={getPos(i)}
        x2={boardSize - padding}
        y2={getPos(i)}
        stroke="var(--wc-text)"
        stroke-width="1"
        opacity="0.85"
      />
    {/each}
    <!-- Vertical lines -->
    {#each Array(size) as _, i (i)}
      <line
        x1={getPos(i)}
        y1={padding}
        x2={getPos(i)}
        y2={boardSize - padding}
        stroke="var(--wc-text)"
        stroke-width="1"
        opacity="0.85"
      />
    {/each}

    <!-- Star Points -->
    {#each starPoints as pt, idx (idx)}
      <circle
        cx={getPos(pt.x)}
        cy={getPos(pt.y)}
        r="3.5"
        fill="var(--wc-text)"
      />
    {/each}

    <!-- Ghost Stone on hover -->
    {#if interactive && hoverIntersection}
      <circle
        cx={getPos(hoverIntersection.x)}
        cy={getPos(hoverIntersection.y)}
        r={step * 0.46}
        fill={turnColor === 1 ? "black" : "white"}
        opacity="0.5"
        stroke={turnColor === 1 ? "none" : "#999"}
        stroke-width="1"
      />
    {/if}

    <!-- Placed Stones -->
    {#each board as row, y (y)}
      {#each row as cell, x (x)}
        {#if cell === 1}
          <!-- Black stone with a radial gradient and outline -->
          <circle
            cx={getPos(x)}
            cy={getPos(y)}
            r={step * 0.46}
            fill="url(#blackStoneGrad)"
            stroke="var(--wc-border)"
            stroke-width="0.5"
            filter="url(#shadow)"
          />
        {:else if cell === 2}
          <!-- White stone with a radial gradient and outline -->
          <circle
            cx={getPos(x)}
            cy={getPos(y)}
            r={step * 0.46}
            fill="url(#whiteStoneGrad)"
            stroke="var(--wc-border)"
            stroke-width="0.5"
            filter="url(#shadow)"
          />
        {/if}
      {/each}
    {/each}

    <!-- Highlight last move (Constant Pulse Ring Animation) -->
    {#if lastMove && lastMove.x >= 0 && lastMove.x < size && lastMove.y >= 0 && lastMove.y < size}
      {@const stoneColor = board[lastMove.y][lastMove.x]}
      {#if stoneColor !== 0}
        <!-- Core highlight center dot -->
        <circle
          cx={getPos(lastMove.x)}
          cy={getPos(lastMove.y)}
          r="3.5"
          fill="var(--wc-accent-warm)"
        />
        <!-- Pulsing radial ring -->
        <circle
          cx={getPos(lastMove.x)}
          cy={getPos(lastMove.y)}
          fill="none"
          stroke="var(--wc-accent-warm)"
          class="em-board-pulse-ring"
        />
      {/if}
    {/if}

    <!-- SVG Definitions (gradients, filters) -->
    <defs>
      <!-- Black stone gradient (Matte dark pastel) -->
      <radialGradient id="blackStoneGrad" cx="30%" cy="30%" r="70%">
        <stop offset="0%" stop-color="var(--wc-text-light)" />
        <stop offset="35%" stop-color="var(--wc-go-black)" />
        <stop offset="100%" stop-color="var(--wc-shadow-dark)" />
      </radialGradient>

      <!-- White stone gradient (Matte pastel white) -->
      <radialGradient id="whiteStoneGrad" cx="30%" cy="30%" r="70%">
        <stop offset="0%" stop-color="#ffffff" />
        <stop offset="75%" stop-color="var(--wc-go-white)" />
        <stop offset="100%" stop-color="var(--wc-border)" />
      </radialGradient>

      <!-- Stone shadow drop (solid crisp look) -->
      <filter id="shadow" x="-10%" y="-10%" width="120%" height="120%">
        <feDropShadow dx="1.5" dy="2.5" stdDeviation="0.5" flood-color="var(--wc-text)" flood-opacity="0.25" />
      </filter>
    </defs>
  </svg>
</div>

<style>
  .board-container {
    width: 100%;
    max-width: min(78vh, 720px);
    margin: 0 auto;
    border-radius: 0px !important;
    background-color: var(--wc-surface);
    border: 2px solid var(--wc-text) !important;
    box-shadow: 6px 6px 0px var(--wc-text) !important; /* ソリッドなポップ影 */
    padding: 12px;
    box-sizing: border-box;
    transition: transform 0.2s ease, box-shadow 0.2s ease;
    touch-action: none; /* タップ遅延防止 */
  }

  .board-container:hover {
    transform: translate(-1px, -1px);
    box-shadow: 7px 7px 0px var(--wc-text) !important;
  }

  /* モバイル画面用の最適化 */
  @media only screen and (max-width: 600px) {
    .board-container {
      padding: 6px;
      box-shadow: 4px 4px 0px var(--wc-text) !important;
      max-width: 100%;
    }
  }

  .go-board {
    width: 100%;
    height: 100%;
    display: block;
    cursor: default;
    user-select: none;
  }
</style>
