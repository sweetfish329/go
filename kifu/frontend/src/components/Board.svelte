<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let board: number[][] = []; // 2D array representing board state
  export let size: number = 19;  // Board size (9, 13, 19)
  export let lastMove: { x: number; y: number } | null = null; // { x, y } of the last placed stone
  export let interactive: boolean = true; // Can place stones by clicking
  export let turnColor: number = 1; // Current player color (1: Black, 2: White) for hover ghost stone

  const dispatch = createEventDispatcher<{
    intersectionClick: { x: number; y: number };
  }>();
  let svgElement: SVGSVGElement;
  let hoverIntersection: { x: number; y: number } | null = null; // { x, y } under mouse pointer

  const padding = 25;
  const boardSize = 500;
  const usableSize = boardSize - padding * 2;
  $: step = usableSize / (size - 1);

  interface StarPoint {
    x: number;
    y: number;
  }

  // Generate star point coordinates
  $: starPoints = getStarPoints(size);

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
      dispatch('intersectionClick', { x, y });
    }
  }

  // Calculate coordinates on mouse move for hover effect
  function handleMouseMove(event: MouseEvent): void {
    if (!interactive || !svgElement) return;

    const rect = svgElement.getBoundingClientRect();
    const clientX = event.clientX - rect.left;
    const clientY = event.clientY - rect.top;

    const svgX = (clientX / rect.width) * boardSize;
    const svgY = (clientY / rect.height) * boardSize;

    const x = Math.round((svgX - padding) / step);
    const y = Math.round((svgY - padding) / step);

    if (x >= 0 && x < size && y >= 0 && y < size && board[y][x] === 0) {
      hoverIntersection = { x, y };
    } else {
      hoverIntersection = null;
    }
  }

  function handleMouseLeave(): void {
    hoverIntersection = null;
  }
</script>

<div class="board-container z-depth-2">
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <svg
    bind:this={svgElement}
    viewBox="0 0 {boardSize} {boardSize}"
    class="go-board"
    on:click={handleSvgClick}
    on:mousemove={handleMouseMove}
    on:mouseleave={handleMouseLeave}
  >
    <!-- Board wood background -->
    <rect width={boardSize} height={boardSize} fill="url(#boardWood)" rx="8" />
    <rect width={boardSize} height={boardSize} fill="none" stroke="#8d6e63" stroke-width="3" rx="8" />

    <!-- Grid lines -->
    <!-- Horizontal lines -->
    {#each Array(size) as _, i}
      <line
        x1={padding}
        y1={getPos(i)}
        x2={boardSize - padding}
        y2={getPos(i)}
        stroke="#4e342e"
        stroke-width="1.2"
      />
    {/each}
    <!-- Vertical lines -->
    {#each Array(size) as _, i}
      <line
        x1={getPos(i)}
        y1={padding}
        x2={getPos(i)}
        y2={boardSize - padding}
        stroke="#4e342e"
        stroke-width="1.2"
      />
    {/each}

    <!-- Star Points -->
    {#each starPoints as pt}
      <circle
        cx={getPos(pt.x)}
        cy={getPos(pt.y)}
        r="4"
        fill="#4e342e"
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
    {#each board as row, y}
      {#each row as cell, x}
        {#if cell === 1}
          <!-- Black stone with a subtle radial gradient for volume -->
          <circle
            cx={getPos(x)}
            cy={getPos(y)}
            r={step * 0.46}
            fill="url(#blackStoneGrad)"
            filter="url(#shadow)"
          />
        {:else if cell === 2}
          <!-- White stone with a subtle radial gradient -->
          <circle
            cx={getPos(x)}
            cy={getPos(y)}
            r={step * 0.46}
            fill="url(#whiteStoneGrad)"
            stroke="#ccc"
            stroke-width="0.5"
            filter="url(#shadow)"
          />
        {/if}
      {/each}
    {/each}

    <!-- Highlight last move -->
    {#if lastMove && lastMove.x >= 0 && lastMove.x < size && lastMove.y >= 0 && lastMove.y < size}
      {@const stoneColor = board[lastMove.y][lastMove.x]}
      {#if stoneColor !== 0}
        <circle
          cx={getPos(lastMove.x)}
          cy={getPos(lastMove.y)}
          r="4"
          fill={stoneColor === 1 ? "#ff5252" : "#d32f2f"}
        />
        <circle
          cx={getPos(lastMove.x)}
          cy={getPos(lastMove.y)}
          r={step * 0.2}
          fill="none"
          stroke={stoneColor === 1 ? "#ff5252" : "#d32f2f"}
          stroke-width="1.5"
        />
      {/if}
    {/if}

    <!-- SVG Definitions (gradients, filters) -->
    <defs>
      <!-- Wood pattern gradient -->
      <linearGradient id="boardWood" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" stop-color="#f5cc84" />
        <stop offset="50%" stop-color="#eac075" />
        <stop offset="100%" stop-color="#dfb466" />
      </linearGradient>

      <!-- Black stone 3D gradient -->
      <radialGradient id="blackStoneGrad" cx="30%" cy="30%" r="70%">
        <stop offset="0%" stop-color="#555555" />
        <stop offset="15%" stop-color="#2a2a2a" />
        <stop offset="100%" stop-color="#111111" />
      </radialGradient>

      <!-- White stone 3D gradient -->
      <radialGradient id="whiteStoneGrad" cx="30%" cy="30%" r="70%">
        <stop offset="0%" stop-color="#ffffff" />
        <stop offset="60%" stop-color="#fdfdfd" />
        <stop offset="100%" stop-color="#e0e0e0" />
      </radialGradient>

      <!-- Stone shadow drop -->
      <filter id="shadow" x="-10%" y="-10%" width="120%" height="120%">
        <feDropShadow dx="1" dy="2" stdDeviation="1.5" flood-color="#000000" flood-opacity="0.35" />
      </filter>
    </defs>
  </svg>
</div>

<style>
  .board-container {
    width: 100%;
    max-width: 550px;
    margin: 0 auto;
    border-radius: 8px;
    background-color: #8d6e63;
    padding: 6px;
    box-sizing: border-box;
  }

  .go-board {
    width: 100%;
    height: 100%;
    display: block;
    cursor: default;
    user-select: none;
  }
</style>
