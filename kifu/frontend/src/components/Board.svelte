<script lang="ts">
  let {
    board = [],
    size = 19,
    lastMove = null,
    interactive = true,
    turnColor = 1,
    candidates = [],
    previewStones = [],
    influenceMap = [],
    showInfluence = false,
    deadStones = [],
    showDeadStones = false,
    onIntersectionClick,
    onCandidateHover,
    onCandidateClick
  } = $props<{
    board?: number[][];
    size?: number;
    lastMove?: { x: number; y: number } | null;
    interactive?: boolean;
    turnColor?: number;
    candidates?: {
      x: number;
      y: number;
      scoreLead: number;
      loss: number;
      winrate: number;
      rank?: number;
      isBest?: boolean;
      coords: string;
    }[];
    previewStones?: {
      x: number;
      y: number;
      color: number;
      stepNumber: number;
    }[];
    influenceMap?: number[][];
    showInfluence?: boolean;
    deadStones?: { x: number; y: number }[];
    showDeadStones?: boolean;
    onIntersectionClick?: (detail: { x: number; y: number }) => void;
    onCandidateHover?: (candidate: any | null) => void;
    onCandidateClick?: (candidate: any) => void;
  }>();

  function getCandidateColor(cand: any): string {
    if (cand.isBest) return "#2196F3"; // Blue for best
    if (cand.loss < 0.5) return "#4CAF50"; // Green for minor loss
    if (cand.loss < 2.0) return "#FFEB3B"; // Yellow for moderate loss
    return "#F44336"; // Red for major loss
  }

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

    // Reset candidate hover when clicking other board areas
    onCandidateHover?.(null);

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

  function handleSvgKeyDown(e: KeyboardEvent): void {
    if ((e.key === 'Enter' || e.key === ' ') && hoverIntersection) {
      e.preventDefault();
      onIntersectionClick?.({ x: hoverIntersection.x, y: hoverIntersection.y });
    }
  }

  function handleCandidateKeyDown(e: KeyboardEvent, cand: any): void {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      e.stopPropagation();
      onCandidateClick?.(cand);
    }
  }
</script>

<div
  class="board-container"
  tabindex={interactive ? 0 : -1}
  role="button"
  aria-label="碁盤。マウスで交点を選択するか、Enterまたはスペースキーで選択中の交点に石を置きます。"
  onclick={handleSvgClick}
  onmousemove={handleMouseMove}
  onmouseleave={handleMouseLeave}
  onkeydown={handleSvgKeyDown}
>
  <svg
    bind:this={svgElement}
    viewBox="0 0 {boardSize} {boardSize}"
    class="go-board"
    aria-hidden="true"
  >
    <!-- Board background (Ash Wood / Kaya surface with subtle natural lighting gradient) -->
    <rect width={boardSize} height={boardSize} fill="url(#boardSurfaceGrad)" rx="2" />
    <!-- Subtle inner bevel highlight line for craftsmanship wood depth -->
    <rect x="1.5" y="1.5" width={boardSize - 3} height={boardSize - 3} fill="none" stroke="rgba(255, 255, 255, 0.45)" stroke-width="1" rx="1.5" />
    <rect width={boardSize} height={boardSize} fill="none" stroke="var(--wc-board-border, var(--wc-text))" stroke-width="3" rx="2" />

    <!-- Grid lines -->
    <!-- Horizontal lines -->
    {#each Array(size) as _, i (i)}
      <line
        x1={padding}
        y1={getPos(i)}
        x2={boardSize - padding}
        y2={getPos(i)}
        stroke="var(--wc-board-line, var(--wc-text))"
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
        stroke="var(--wc-board-line, var(--wc-text))"
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
        fill="var(--wc-board-star, var(--wc-text))"
      />
    {/each}

    <!-- Influence Map Overlay -->
    {#if showInfluence && influenceMap && influenceMap.length > 0}
      {#each Array(size) as _, y (y)}
        {#each Array(size) as _, x (x)}
          {@const val = influenceMap[y]?.[x] || 0}
          {#if val !== 0}
            <circle
              cx={getPos(x)}
              cy={getPos(y)}
              r={step * 0.43}
              fill={val > 0 ? "#111111" : "#ffffff"}
              opacity={val > 0 ? Math.min(Math.abs(val) * 0.4, 0.55) : Math.min(Math.abs(val) * 0.4, 0.65)}
              stroke={val > 0 ? "rgba(0,0,0,0.15)" : "rgba(0,0,0,0.3)"}
              stroke-width="0.5"
            />
          {/if}
        {/each}
      {/each}
    {/if}

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
          {@const isDead = showDeadStones && deadStones && deadStones.some((s: { x: number; y: number }) => s.x === x && s.y === y)}
          <!-- Black stone with a radial gradient and outline -->
          <g opacity={isDead ? 0.35 : 1.0}>
            <circle
              cx={getPos(x)}
              cy={getPos(y)}
              r={step * 0.46}
              fill="url(#blackStoneGrad)"
              stroke="var(--wc-go-black-stroke, var(--wc-border))"
              stroke-width="0.5"
              filter="url(#shadow)"
            />
            {#if isDead}
              <!-- Red X mark for dead stones -->
              <line 
                x1={getPos(x) - step * 0.22} 
                y1={getPos(y) - step * 0.22} 
                x2={getPos(x) + step * 0.22} 
                y2={getPos(y) + step * 0.22} 
                stroke="#d32f2f" 
                stroke-width="2.5" 
                stroke-linecap="round"
              />
              <line 
                x1={getPos(x) + step * 0.22} 
                y1={getPos(y) - step * 0.22} 
                x2={getPos(x) - step * 0.22} 
                y2={getPos(y) + step * 0.22} 
                stroke="#d32f2f" 
                stroke-width="2.5" 
                stroke-linecap="round"
              />
            {/if}
          </g>
        {:else if cell === 2}
          {@const isDead = showDeadStones && deadStones && deadStones.some((s: { x: number; y: number }) => s.x === x && s.y === y)}
          <!-- White stone with a radial gradient and outline -->
          <g opacity={isDead ? 0.35 : 1.0}>
            <circle
              cx={getPos(x)}
              cy={getPos(y)}
              r={step * 0.46}
              fill="url(#whiteStoneGrad)"
              stroke="var(--wc-go-white-stroke, var(--wc-border))"
              stroke-width="0.5"
              filter="url(#shadow)"
            />
            {#if isDead}
              <!-- Red X mark for dead stones -->
              <line 
                x1={getPos(x) - step * 0.22} 
                y1={getPos(y) - step * 0.22} 
                x2={getPos(x) + step * 0.22} 
                y2={getPos(y) + step * 0.22} 
                stroke="#d32f2f" 
                stroke-width="2.5" 
                stroke-linecap="round"
              />
              <line 
                x1={getPos(x) + step * 0.22} 
                y1={getPos(y) - step * 0.22} 
                x2={getPos(x) - step * 0.22} 
                y2={getPos(y) + step * 0.22} 
                stroke="#d32f2f" 
                stroke-width="2.5" 
                stroke-linecap="round"
              />
            {/if}
          </g>
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
          r="24"
          fill="none"
          stroke="var(--wc-accent-warm)"
          stroke-width="1.5"
          class="em-board-pulse-ring"
        />
      {/if}
    {/if}

    <!-- AI Variation Preview Stones -->
    {#if previewStones && previewStones.length > 0}
      {#each previewStones as stone, idx (idx)}
        <g opacity="0.75" filter="url(#shadow)">
          <circle
            cx={getPos(stone.x)}
            cy={getPos(stone.y)}
            r={step * 0.44}
            fill={stone.color === 1 ? "black" : "white"}
            stroke={stone.color === 1 ? "var(--wc-border)" : "#bbb"}
            stroke-width="1"
          />
          <text
            x={getPos(stone.x)}
            y={getPos(stone.y)}
            dy="0.35em"
            text-anchor="middle"
            fill={stone.color === 1 ? "white" : "black"}
            font-size={step * 0.35}
            font-weight="bold"
            font-family="'JetBrains Mono', monospace"
          >
            {stone.stepNumber}
          </text>
        </g>
      {/each}
    {/if}

    <!-- AI Candidate Moves -->
    {#if candidates && candidates.length > 0}
      {#each candidates as cand, idx (idx)}
        <g
          class="board-candidate"
          onmouseenter={() => !isMobileDevice && onCandidateHover?.(cand)}
          onmouseleave={() => !isMobileDevice && onCandidateHover?.(null)}
          onclick={(e) => {
            e.stopPropagation();
            onCandidateClick?.(cand);
          }}
          onkeydown={(e) => handleCandidateKeyDown(e, cand)}
          tabindex="0"
          role="button"
          aria-label="AI候補手 {cand.isBest ? '最善' : cand.rank} (座標: {cand.coords})"
          style="cursor: pointer;"
        >
          <!-- Outer circle -->
          <circle
            cx={getPos(cand.x)}
            cy={getPos(cand.y)}
            r={step * 0.32}
            fill={getCandidateColor(cand)}
            stroke="var(--wc-text)"
            stroke-width="1.5"
            opacity="0.9"
          />
          <!-- Inside text (rank or score loss) -->
          <text
            x={getPos(cand.x)}
            y={getPos(cand.y)}
            dy="0.35em"
            text-anchor="middle"
            fill={cand.loss < 0.5 || cand.isBest ? "white" : "black"}
            font-size={step * 0.28}
            font-weight="bold"
            font-family="'JetBrains Mono', monospace"
          >
            {cand.isBest ? 'A' : (cand.rank || '')}
          </text>
        </g>
      {/each}
    {/if}

    <!-- SVG Definitions (gradients, filters) -->
    <defs>
      <!-- Board wood grain subtle gradient -->
      <linearGradient id="boardSurfaceGrad" x1="0%" y1="0%" x2="0%" y2="100%">
        <stop offset="0%" stop-color="var(--wc-board-bg-top, #f2e5d2)" />
        <stop offset="100%" stop-color="var(--wc-board-bg-bottom, #e6d3b7)" />
      </linearGradient>

      <!-- Black stone gradient (Matte dark pastel / Lacquered Kuroishi) -->
      <radialGradient id="blackStoneGrad" cx="30%" cy="30%" r="70%">
        <stop offset="0%" stop-color="var(--wc-go-black-grad-0, #4a5c55)" />
        <stop offset="35%" stop-color="var(--wc-go-black-grad-35, #18221e)" />
        <stop offset="100%" stop-color="var(--wc-go-black-grad-100, #0a0f0d)" />
      </radialGradient>

      <!-- White stone gradient (Matte pastel white / Smooth Shiroishi) -->
      <radialGradient id="whiteStoneGrad" cx="30%" cy="30%" r="70%">
        <stop offset="0%" stop-color="var(--wc-go-white-grad-0, #ffffff)" />
        <stop offset="75%" stop-color="var(--wc-go-white-grad-75, #f8faf9)" />
        <stop offset="100%" stop-color="var(--wc-go-white-grad-100, #c8d2cd)" />
      </radialGradient>

      <!-- Stone shadow drop (solid crisp look, never glow white in dark mode) -->
      <filter id="shadow" x="-15%" y="-15%" width="130%" height="130%">
        <feDropShadow dx="1.2" dy="2.0" stdDeviation="0.7" flood-color="var(--wc-board-shadow, rgba(0, 0, 0, 0.4))" flood-opacity="0.38" />
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
    border: 2px solid var(--wc-border-solid, var(--wc-text)) !important;
    box-shadow: 6px 6px 0px var(--wc-shadow-solid, var(--wc-text)) !important; /* ソリッドなポップ影 */
    padding: 12px;
    box-sizing: border-box;
    transition: transform 0.2s ease, box-shadow 0.2s ease;
    touch-action: none; /* タップ遅延防止 */
  }

  .board-container:hover {
    transform: translate(-1px, -1px);
    box-shadow: 7px 7px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
  }

  /* モバイル画面用の最適化 */
  @media only screen and (max-width: 600px) {
    .board-container {
      padding: 6px;
      box-shadow: 4px 4px 0px var(--wc-shadow-solid, var(--wc-text)) !important;
      max-width: 100%;
    }
  }

  /* ダークモード時の和モダン額縁スタイル */
  :global(:root.dark) .board-container {
    background-color: var(--wc-surface);
    border: 2px solid var(--wc-border) !important;
    box-shadow: 6px 6px 0px var(--wc-shadow-dark) !important;
  }

  :global(:root.dark) .board-container:hover {
    box-shadow: 7px 7px 0px var(--wc-shadow-dark) !important;
  }

  @media only screen and (max-width: 600px) {
    :global(:root.dark) .board-container {
      box-shadow: 4px 4px 0px var(--wc-shadow-dark) !important;
    }
  }

  @media (prefers-color-scheme: dark) {
    :global(:root:not(.light)) .board-container {
      background-color: var(--wc-surface);
      border: 2px solid var(--wc-border) !important;
      box-shadow: 6px 6px 0px var(--wc-shadow-dark) !important;
    }
    :global(:root:not(.light)) .board-container:hover {
      box-shadow: 7px 7px 0px var(--wc-shadow-dark) !important;
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
