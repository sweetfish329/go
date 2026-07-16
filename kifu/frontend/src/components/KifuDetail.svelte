<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Board from './Board.svelte';
  import ShareDialog from './ShareDialog.svelte';
  import { SgfPlayer, stringifySgf } from '../lib/sgfPlayer';
  import type { SgfNode } from '../lib/sgfPlayer';
  import { auth } from '../lib/auth.svelte';
  import { coordsToBoard } from '../lib/aiAnalysis';
  import { sgfToCoords, getInfluenceMap, getDeadStones } from '../lib/goEngine';

  let {
    kifuId = "",
    shareToken = "",
    userId = "",
    onBack = () => {}
  } = $props<{
    kifuId?: string;
    shareToken?: string;
    userId?: string;
    onBack?: () => void;
  }>();

  let showShareDialog = $state(false);

  interface KifuDetailData {
    id: string;
    title: string;
    sgf_data: string;
    handicap: number;
    komi: number;
    black_player?: string;
    black_rank?: string;
    white_player?: string;
    white_rank?: string;
    result?: string;
    game_date?: string;
    share_token?: string;
    share_expires_at?: string;
    uploaded_by?: string;
    is_private?: boolean;
  }

  interface ReviewItem {
    id: string;
    move_number: number;
    reviewer_name: string;
    comment: string;
    variations?: string;
  }

  interface CommentItem {
    author: string;
    text: string;
    reviewId?: string;
    reviewerName?: string;
    isFromVariationRoot?: boolean;
  }

  interface BranchItem {
    label: string;
    node: SgfNode;
    originalIndex: number;
  }

  let kifu = $state.raw<KifuDetailData | null>(null);
  $effect(() => {
    if (kifu) {
      const parts: string[] = [];
      if (kifu.title) {
        parts.push(kifu.title);
      }
      if (kifu.black_player && kifu.white_player) {
        parts.push(`${kifu.black_player} vs ${kifu.white_player}`);
      }
      parts.push("kifu_store");
      document.title = parts.join(" | ");
    } else {
      document.title = "棋譜詳細 | kifu_store";
    }
  });
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Player state
  let player = $state.raw<SgfPlayer | null>(null);
  let boardState = $state.raw<number[][]>([]);
  let lastMove = $state<{ x: number; y: number } | null>(null);
  let captives = $state({ B: 0, W: 0 });
  let currentIndex = $state(0);
  let maxIndex = $state(0);
  let comments = $state.raw<CommentItem[]>([]); // Comments at current move
  let alternativeBranches = $state.raw<BranchItem[]>([]); // Sibling nodes (alternative moves)

  // AI analysis state
  let showAiAnalysis = $state(true);
  let hasAiAnalysis = $state(false);
  let aiCandidates = $state.raw<any[]>([]);
  let aiPreviewStones = $state.raw<any[]>([]);
  let activeHoveredCandidate = $state<any | null>(null);
  let graphMode = $state<'winrate' | 'score'>('winrate');
  let graphHoverIndex = $state<number | null>(null);
  let aiData = $state<{ moveNumber: number; winrate: number; scoreLead: number }[]>([]);

  const isMobileDevice = $derived(typeof window !== 'undefined' && 
    (/Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent) ||
     window.innerWidth < 768 || 
     window.matchMedia('(pointer: coarse)').matches));



  // Autoplay state
  let autoplayInterval = $state<any>(null);
  let isAutoplay = $state(false);
  let autoplaySpeed = $state(2000); // ms

  // Review mode state
  let reviewMode = $state(false);
  let reviewerName = $state("");
  let reviewComment = $state("");
  let isAddingReview = $state(false);
  let reviewList = $state.raw<ReviewItem[]>([]); // Review items fetched from database
  let isViewingVariation = $state(false);
  let activeReviewer = $state("");
  let isGameInfoExpanded = $state(false);
  let editingReviewId = $state<string | null>(null);

  // For board config
  let boardSize = $state(19);
  let currentTurn = $state(1); // 1: Black, 2: White (used for review mode placing stones)

  // Sabaki features state
  let showInfluenceMap = $state(false);
  let showDeadStones = $state(false);

  const influenceMap = $derived(
    showInfluenceMap && boardState.length > 0
      ? getInfluenceMap(boardState, boardSize)
      : []
  );

  let deadStones = $state<{ x: number; y: number }[]>([]);

  $effect(() => {
    let active = true;
    if (showDeadStones && boardState.length > 0) {
      getDeadStones(boardState, boardSize).then((res) => {
        if (active) {
          deadStones = res;
        }
      }).catch((err) => {
        if (active) {
          console.error("Failed to guess dead stones:", err);
          deadStones = [];
        }
      });
    } else {
      deadStones = [];
    }

    return () => {
      active = false;
    };
  });

  // 表示オプションモーダル
  let showSettingsModal = $state(false);

  const isPublicProfileMode = $derived(!!userId && !!kifuId);
  const isOwner = $derived(!!kifu && auth.isLoggedIn && kifu.uploaded_by === auth.userId);
  const isPublic = $derived(!!kifu && kifu.is_private === false);

  $effect(() => {
    if (isPublic) {
      reviewMode = false;
    }
  });

  // Type helper for Materialize global object
  const getM = () => (window as any).M;

  async function loadKifu() {
    loading = true;
    try {
      let kifuRes: Response;
      let reviewRes: Response;

      if (isPublicProfileMode) {
        kifuRes = await fetch(`/api/u/${userId}/kifus/${kifuId}`);
        if (!kifuRes.ok) throw new Error("Failed to fetch public kifu details");
        kifu = await kifuRes.json();

        reviewRes = await fetch(`/api/u/${userId}/kifus/${kifuId}/reviews`);
      } else if (shareToken) {
        kifuRes = await fetch(`/api/share/${shareToken}`);
        if (!kifuRes.ok) throw new Error("Failed to fetch shared kifu details");
        kifu = await kifuRes.json();

        reviewRes = await fetch(`/api/share/${shareToken}/reviews`);
      } else {
        kifuRes = await fetch(`/api/kifus/${kifuId}`, {
          headers: auth.getHeaders()
        });
        if (!kifuRes.ok) throw new Error("Failed to fetch kifu details");
        kifu = await kifuRes.json();

        reviewRes = await fetch(`/api/kifus/${kifuId}/reviews`, {
          headers: auth.getHeaders()
        });
      }

      if (!kifu) throw new Error("Kifu data is null");
      boardSize = kifu.handicap > 0 || kifu.sgf_data.includes("SZ[19]") ? 19 : 19;
      
      if (reviewRes.ok) {
        reviewList = await reviewRes.json();
      }

      // Initialize SgfPlayer
      player = new SgfPlayer(kifu.sgf_data, boardSize);
      
      // Merge review comments and variations into SGF tree
      mergeReviewsIntoPlayer();

      // Extract AI Analysis data
      extractAiData();
      hasAiAnalysis = aiData.length > 0;

      updatePlayerState();
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  // Recursively marks SGF nodes as variation branch
  function markAsVariation(node: SgfNode, reviewerName: string) {
    (node as any).is_variation = true;
    (node as any).reviewer_name = reviewerName;
    for (const child of node.children) {
      markAsVariation(child, reviewerName);
    }
  }

  // Merges comments and variation subtrees from Database into the active SgfPlayer tree
  function mergeReviewsIntoPlayer() {
    if (!player || !reviewList || reviewList.length === 0) return;

    for (const rev of reviewList) {
      // Find the parent node in SGF based on move_number
      let parentNode: SgfNode | null = player.root;
      let count = 0;
      
      // Traverse to the targeted move_number
      while (parentNode && count < rev.move_number) {
        if (parentNode.children.length > 0) {
          parentNode = parentNode.children[0];
          count++;
        } else {
          break;
        }
      }

      if (parentNode && count === rev.move_number) {
        // Add variation if present
        if (rev.variations && rev.variations.trim() !== "") {
          try {
            const varPlayer = new SgfPlayer(rev.variations, boardSize);
            if (varPlayer.root) {
              // Ensure it's not already added
              const targetProps = JSON.stringify(varPlayer.root.properties);
              const alreadyExists = parentNode.children.some(child => {
                return JSON.stringify(child.properties) === targetProps;
              });

              // Store the DB Review ID on the variation node for edit tracking
              (varPlayer.root as any).review_id = rev.id;

              if (!alreadyExists) {
                // Attach comment directly onto the variation root node
                if (rev.comment && rev.comment.trim() !== "") {
                  if (!varPlayer.root.properties["C"]) {
                    varPlayer.root.properties["C"] = [];
                  }
                  varPlayer.root.properties["C"].push(`${rev.reviewer_name}: ${rev.comment}`);
                }

                markAsVariation(varPlayer.root, rev.reviewer_name);
                varPlayer.root.parent = parentNode;
                parentNode.children.push(varPlayer.root);
              } else {
                // If already exists, find existing and update attributes
                const existingChild = parentNode.children.find(child => JSON.stringify(child.properties) === targetProps);
                if (existingChild) {
                  (existingChild as any).review_id = rev.id;
                  if (rev.comment && rev.comment.trim() !== "") {
                    existingChild.properties["C"] = [`${rev.reviewer_name}: ${rev.comment}`];
                  }
                }
              }
            }
          } catch (e) {
            console.error("Failed to parse variation SGF:", e);
          }
        } else {
          // No variation, just comment on the main path node itself
          if (!parentNode.properties["C"]) {
            parentNode.properties["C"] = [];
          }
          const commentText = `${rev.reviewer_name}: ${rev.comment}`;
          if (!parentNode.properties["C"].includes(commentText)) {
            parentNode.properties["C"].push(commentText);
          }
          (parentNode as any).review_id = rev.id;
        }
      }
    }
    
    // Recalculate main path after merge
    player.initGame();
  }

  function updatePlayerState() {
    if (!player) return;
    const state = player.getCurrentState();
    boardState = state.board;
    lastMove = state.lastMove;
    captives = state.captives;
    currentIndex = player.currentIndex;
    maxIndex = player.history.length - 1;

    // Check if currently on a variation branch and locate its root
    const currentNode = state.node;
    let variationFound = false;
    let reviewer = "";
    let variationRootNode: SgfNode | null = null;

    if (currentNode) {
      let temp: SgfNode | null = currentNode;
      while (temp) {
        if ((temp as any).is_variation) {
          variationFound = true;
          reviewer = (temp as any).reviewer_name || "";
          variationRootNode = temp; // Keep traversing up to find the highest variation node (the root)
        }
        temp = temp.parent;
      }
    }
    isViewingVariation = variationFound;
    activeReviewer = reviewer;

    // Get current comment from node
    comments = [];
    if (currentNode) {
      // Find review ID associated with this node or parent (if we are in a variation, the review ID is on the variation root)
      let associatedReviewId = (currentNode as any).review_id || "";
      let associatedReviewer = (currentNode as any).reviewer_name || "";
      
      // If we don't have review_id on current node, traverse up to find it in the variation path
      if (!associatedReviewId && isViewingVariation) {
        let temp: SgfNode | null = currentNode;
        while (temp) {
          if ((temp as any).review_id) {
            associatedReviewId = (temp as any).review_id;
            associatedReviewer = (temp as any).reviewer_name || "";
            break;
          }
          temp = temp.parent;
        }
      }

      // 1. Add current node's comments
      if (currentNode.properties["C"]) {
        for (const rawComment of currentNode.properties["C"]) {
          const colonIndex = rawComment.indexOf(":");
          let author = "コメント";
          let text = rawComment;
          if (colonIndex !== -1) {
            author = rawComment.substring(0, colonIndex).trim();
            text = rawComment.substring(colonIndex + 1).trim();
          }
          
          comments.push({
            author: author,
            text: text,
            reviewId: associatedReviewId,
            reviewerName: associatedReviewer,
            isFromVariationRoot: false
          });
        }
      }

      // 2. If in a variation and current node is not the variation root,
      // bring the variation root comment forward so it stays visible.
      if (isViewingVariation && variationRootNode && variationRootNode !== currentNode) {
        if (variationRootNode.properties["C"]) {
          for (const rawComment of variationRootNode.properties["C"]) {
            const colonIndex = rawComment.indexOf(":");
            let author = "コメント";
            let text = rawComment;
            if (colonIndex !== -1) {
              author = rawComment.substring(0, colonIndex).trim();
              text = rawComment.substring(colonIndex + 1).trim();
            }
            
            // Avoid duplicate text
            const exists = comments.some(c => c.text === text && c.author === author);
            if (!exists) {
              comments.push({
                author: author,
                text: text,
                reviewId: associatedReviewId,
                reviewerName: associatedReviewer,
                isFromVariationRoot: true
              });
            }
          }
        }
      }
    }

    // Determine current turn color for review mode placing stones
    // Default: alternate based on move number
    currentTurn = currentIndex % 2 === 0 ? 1 : 2; // Black on even index (0 is handicap/root, 1 is Black first move if no handicap)
    // Adjust if handicap exists
    if (kifu && kifu.handicap > 0) {
      currentTurn = currentIndex % 2 === 0 ? 2 : 1; // White on even index (0 is AB, 1 is White first move)
    }

    // Get alternative branches (siblings)
    // Filter out primary path nodes (only show actual variation review branches)
    alternativeBranches = player.getAlternativeBranches()
      .filter((node) => (node as any).is_variation === true)
      .map((node) => {
        let moveLabel = "変化図";
        if (node.properties.B) moveLabel = `黒 ${node.properties.B[0]}`;
        else if (node.properties.W) moveLabel = `白 ${node.properties.W[0]}`;
        
        const nodeReviewer = (node as any).reviewer_name || "";
        const label = nodeReviewer ? `${nodeReviewer} さんの指導 (${moveLabel})` : `変化図 (${moveLabel})`;
        
        let originalIndex = -1;
        if (node.parent) {
          originalIndex = node.parent.children.indexOf(node);
        }
        
        return {
          label: label,
          node: node,
          originalIndex: originalIndex
        };
      });

    // AI Candidates calculation
    let cands: any[] = [];
    if (showAiAnalysis && player) {
      const nextEntry = player.history[currentIndex + 1];
      const nextNode = nextEntry?.node;
      if (nextNode && nextNode.aiAnalysis) {
        const ai = nextNode.aiAnalysis;
        
        // 1. Best Move
        if (ai.bestMoveCoords) {
          cands.push({
            x: ai.bestMoveCoords.x,
            y: ai.bestMoveCoords.y,
            scoreLead: ai.scoreLead + (ai.loss || 0),
            loss: 0,
            winrate: ai.winrate,
            isBest: true,
            coords: ai.bestMove,
            rank: 1,
            variation: ai.variation
          });
        }
        
        // 2. Played Move
        let playedMoveCoords: any = null;
        if (nextNode.properties.B) {
          playedMoveCoords = sgfToCoords(nextNode.properties.B[0]);
        } else if (nextNode.properties.W) {
          playedMoveCoords = sgfToCoords(nextNode.properties.W[0]);
        }
        
        if (playedMoveCoords && !playedMoveCoords.pass && playedMoveCoords.x !== undefined && playedMoveCoords.y !== undefined) {
          const isAlreadyBest = cands.some(c => c.x === playedMoveCoords.x && c.y === playedMoveCoords.y);
          if (!isAlreadyBest) {
            cands.push({
              x: playedMoveCoords.x,
              y: playedMoveCoords.y,
              scoreLead: ai.scoreLead,
              loss: ai.loss || 0,
              winrate: ai.winrate,
              isBest: false,
              coords: coordsToBoard(playedMoveCoords.x, playedMoveCoords.y),
              rank: ai.rank || 2
            });
          }
        }
      }
    }
    aiCandidates = cands;

    // Reset preview stones on move change
    aiPreviewStones = [];
    activeHoveredCandidate = null;
  }

  function handleExportPath() {
    if (!player) return;
    const sgfStr = player.exportCurrentPathAsSgf();
    if (!sgfStr) {
      getM().toast({ html: 'SGF出力に失敗しました', classes: 'red' });
      return;
    }

    navigator.clipboard.writeText(sgfStr).then(() => {
      getM().toast({ html: '現在の手順（SGF）をコピーしました！', classes: 'green' });
    }).catch(err => {
      console.error('Failed to copy SGF:', err);
      getM().toast({ html: 'コピーに失敗しました', classes: 'red' });
    });
  }

  function extractAiData() {
    if (!player) return;
    const data: typeof aiData = [];
    player.history.forEach((entry, idx) => {
      if (idx > 0 && entry.node && entry.node.aiAnalysis) {
        data.push({
          moveNumber: idx,
          winrate: entry.node.aiAnalysis.winrate,
          scoreLead: entry.node.aiAnalysis.scoreLead
        });
      }
    });
    aiData = data;
  }

  function handleCandidateHover(cand: any | null) {
    activeHoveredCandidate = cand;
    if (cand && cand.variation && cand.variation.length > 0) {
      aiPreviewStones = cand.variation.map((v: any, idx: number) => ({
        x: v.x,
        y: v.y,
        color: v.color,
        stepNumber: idx + 1
      }));
    } else {
      aiPreviewStones = [];
    }
  }

  function handleCandidateClick(cand: any) {
    if (!player) return;

    if (isMobileDevice && activeHoveredCandidate !== cand) {
      handleCandidateHover(cand);
      return;
    }

    // Check if the clicked candidate is actually the played move
    const nextEntry = player.history[currentIndex + 1];
    const nextNode = nextEntry?.node;
    let playedMoveCoords: any = null;
    if (nextNode) {
      if (nextNode.properties.B) playedMoveCoords = sgfToCoords(nextNode.properties.B[0]);
      else if (nextNode.properties.W) playedMoveCoords = sgfToCoords(nextNode.properties.W[0]);
    }

    if (playedMoveCoords && playedMoveCoords.x === cand.x && playedMoveCoords.y === cand.y) {
      stepForward();
    } else {
      // Create variation branch for the candidate
      // Get the color of current turn (alternate B/W)
      const turn = currentIndex % 2 === 0 ? 1 : 2;
      const res = player.addMove(cand.x, cand.y, turn);
      if (res.success) {
        if (res.isNew && res.node) {
          markAsVariation(res.node, "AI分析");
        }
        updatePlayerState();
        
        const M = getM();
        if (M) {
          M.toast({ html: 'AIの候補手変化図に入りました', classes: 'blue darken-2' });
        }
      }
    }
  }

  function handleGraphMouseMove(e: MouseEvent) {
    if (!player || aiData.length === 0) return;
    const container = e.currentTarget as HTMLDivElement;
    const rect = container.getBoundingClientRect();
    const clientX = e.clientX - rect.left;
    const percentage = clientX / rect.width;
    
    let targetMove = Math.round(percentage * maxIndex);
    targetMove = Math.max(1, Math.min(maxIndex, targetMove));
    
    graphHoverIndex = targetMove;
  }

  function handleGraphMouseLeave() {
    graphHoverIndex = null;
  }

  function handleGraphClick() {
    if (!player || graphHoverIndex === null) return;
    player.goTo(graphHoverIndex);
    updatePlayerState();
  }

  function handleGraphKeyDown(e: KeyboardEvent) {
    if (e.key === 'ArrowLeft') {
      e.preventDefault();
      stepBack(1);
    } else if (e.key === 'ArrowRight') {
      e.preventDefault();
      stepForward(1);
    }
  }


  const graphWidth = 400;
  const graphHeight = 100;

  function getCandidateColor(cand: any): string {
    if (cand.isBest) return "#2196F3"; // Blue for best
    if (cand.loss < 0.5) return "#4CAF50"; // Green for minor loss
    if (cand.loss < 2.0) return "#FFEB3B"; // Yellow for moderate loss
    return "#F44336"; // Red for major loss
  }

  // Derived graph coordinates and path
  const graphPoints = $derived.by(() => {
    return aiData.map((d) => {
      const x = (d.moveNumber / maxIndex) * graphWidth;
      let y = 0;
      if (graphMode === 'winrate') {
        y = (1.0 - d.winrate / 100.0) * graphHeight;
      } else {
        const scores = aiData.map(sd => sd.scoreLead);
        const maxScore = Math.max(10, ...scores);
        const minScore = Math.min(-10, ...scores);
        const scoreRange = maxScore - minScore || 1;
        y = graphHeight - ((d.scoreLead - minScore) / scoreRange) * graphHeight;
      }
      return { x, y, d };
    });
  });

  const graphPointsPath = $derived(graphPoints.map(p => `${p.x},${p.y}`).join(' '));
  const currentX = $derived((currentIndex / (maxIndex || 1)) * graphWidth);

  // Navigation handlers
  function goFirst() {
    if (player && player.goTo(0)) updatePlayerState();
  }

  function goLast() {
    if (player && player.goTo(maxIndex)) updatePlayerState();
  }

  function stepBack(steps = 1) {
    if (!player) return;
    let target = Math.max(0, currentIndex - steps);
    if (player.goTo(target)) updatePlayerState();
  }

  // eslint-disable-next-line no-undef
  function stepForward(steps = 1) {
    if (!player) return;
    let target = Math.min(maxIndex, currentIndex + steps);
    if (player.goTo(target)) updatePlayerState();
  }

  function handleSliderChange(e: Event) {
    const target = e.target as HTMLInputElement;
    const idx = parseInt(target.value);
    if (player && player.goTo(idx)) updatePlayerState();
  }

  function selectBranch(branchIndex: number) {
    if (player && player.selectBranch(branchIndex)) {
      updatePlayerState();
    }
  }

  // Autoplay controls
  function toggleAutoplay() {
    if (isAutoplay) {
      stopAutoplay();
    } else {
      startAutoplay();
    }
  }

  function startAutoplay() {
    isAutoplay = true;
    autoplayInterval = setInterval(() => {
      if (currentIndex < maxIndex) {
        stepForward();
      } else {
        stopAutoplay();
      }
    }, autoplaySpeed);
  }

  function stopAutoplay() {
    isAutoplay = false;
    if (autoplayInterval) {
      clearInterval(autoplayInterval);
      autoplayInterval = null;
    }
  }

  // Handle board click (Review / Edit mode)
  async function handleIntersectionClick(detail: { x: number; y: number }) {
    const { x, y } = detail;

    if (!reviewMode) {
      const M = getM();
      if (M) {
        M.toast({ html: '盤面を編集するには「添削モード」を有効にしてください', classes: 'amber darken-2' });
      }
      return;
    }

    if (!player) return;

    // Add move to player
    const res = player.addMove(x, y, currentTurn);
    if (res.success) {
      if (res.isNew && res.node) {
        const reviewer = reviewerName.trim() || auth.username || "あなた";
        markAsVariation(res.node, reviewer);
      }
      updatePlayerState();
      
      // If it created a new branch/variation, notify the user
      if (res.isNew) {
        const M = getM();
        if (M) {
          M.toast({ html: '新しい変化図を作成しました', classes: 'green darken-2' });
        }
      }
    } else {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + res.error, classes: 'red' });
      }
    }
  }

  // Save or Update Review Comment
  async function handleSaveReview() {
    if (!reviewerName.trim() || !reviewComment.trim()) {
      const M = getM();
      if (M) {
        M.toast({ html: '添削者名とコメントを入力してください', classes: 'amber' });
      }
      return;
    }

    if (!player) return;

    isAddingReview = true;
    try {
      const state = player.getCurrentState();
      
      // Extract the variation SGF sub-tree from the branch start point
      let variationsSgf = "";
      let targetMoveNumber = currentIndex;
      const currentNode = state.node;
      
      if (currentNode && currentNode.parent) {
        // Find the root of this variation branch (whose parent is on the primary path)
        let variationRoot = currentNode;
        let temp = currentNode;
        while (temp && temp.parent) {
          if (!(temp.parent as any).is_variation) {
            variationRoot = temp;
            break;
          }
          temp = temp.parent;
        }

        if ((variationRoot as any).is_variation) {
          // Calculate the move number of the parent (primary path node)
          let count = 0;
          let p: SgfNode | null = player.root;
          while (p && p !== variationRoot.parent) {
            if (p.children.length > 0) {
              p = p.children[0];
              count++;
            } else {
              break;
            }
          }
          targetMoveNumber = count;
          variationsSgf = stringifySgf(variationRoot);
        }
      }

      let res: Response;
      const payload = {
        move_number: targetMoveNumber,
        node_path: String(targetMoveNumber),
        reviewer_name: reviewerName.trim(),
        comment: reviewComment.trim(),
        variations: variationsSgf
      };

      if (editingReviewId) {
        // Update existing review
        res = await fetch(`/api/kifus/${kifuId}/reviews/${editingReviewId}`, {
          method: 'PUT',
          headers: auth.getHeaders(),
          body: JSON.stringify(payload)
        });
      } else {
        // Create new review
        if (isPublicProfileMode) {
          res = await fetch(`/api/u/${userId}/kifus/${kifuId}/reviews`, {
            method: 'POST',
            headers: auth.getHeaders(),
            body: JSON.stringify(payload)
          });
        } else if (shareToken) {
          res = await fetch(`/api/share/${shareToken}/reviews`, {
            method: 'POST',
            headers: auth.getHeaders(),
            body: JSON.stringify(payload)
          });
        } else {
          res = await fetch(`/api/kifus/${kifuId}/reviews`, {
            method: 'POST',
            headers: auth.getHeaders(),
            body: JSON.stringify(payload)
          });
        }
      }

      if (!res.ok) {
        const errorData = await res.json().catch(() => ({}));
        throw new Error(errorData.error || "Failed to save review");
      }

      const savedReview = await res.json();
      
      if (editingReviewId) {
        // Replace in reviewList
        reviewList = reviewList.map(r => r.id === editingReviewId ? savedReview : r);
        editingReviewId = null;
      } else {
        reviewList = [...reviewList, savedReview];
      }

      // Re-initialize player with original SGF and re-merge all reviews to ensure clean state
      if (kifu) {
        player = new SgfPlayer(kifu.sgf_data, boardSize);
        mergeReviewsIntoPlayer();
        updatePlayerState();
      }

      // Reset comment text
      reviewComment = "";
      
      const M = getM();
      if (M) {
        M.toast({ html: '添削内容を保存しました！', classes: 'green darken-1' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red' });
      }
    } finally {
      isAddingReview = false;
    }
  }

  function handleEditReview(reviewId: string, reviewerNameVal: string, commentVal: string) {
    editingReviewId = reviewId;
    reviewMode = true;
    reviewerName = reviewerNameVal;
    reviewComment = commentVal;

    // Navigate to the branch point
    const rev = reviewList.find(r => r.id === reviewId);
    if (rev && player) {
      player.goTo(rev.move_number);
      // Select the branch with the correct review_id
      const state = player.getCurrentState();
      if (state.node) {
        const branchIdx = state.node.children.findIndex(child => (child as any).review_id === reviewId);
        if (branchIdx !== -1) {
          player.selectBranch(branchIdx);
        }
      }
      updatePlayerState();
    }

    const M = getM();
    if (M) {
      M.toast({ html: '添削の編集を開始しました。内容や変化手順を修正して「更新」を押してください。', classes: 'amber darken-2' });
    }
  }

  function handleCancelEdit() {
    editingReviewId = null;
    reviewComment = "";
    
    // Re-initialize player to reset unsaved scratch changes
    if (kifu) {
      player = new SgfPlayer(kifu.sgf_data, boardSize);
      mergeReviewsIntoPlayer();
      updatePlayerState();
    }

    const M = getM();
    if (M) {
      M.toast({ html: '編集をキャンセルしました', classes: 'grey darken-1' });
    }
  }

  async function handleDeleteReview(reviewId: string) {
    if (!confirm("この指導コメントと変化図のまとまりを削除してもよろしいですか？")) return;

    try {
      const res = await fetch(`/api/kifus/${kifuId}/reviews/${reviewId}`, {
        method: 'DELETE',
        headers: auth.getHeaders()
      });

      if (!res.ok) {
        const errorData = await res.json().catch(() => ({}));
        throw new Error(errorData.error || "Failed to delete review");
      }

      // Filter out from local list
      reviewList = reviewList.filter(r => r.id !== reviewId);
      
      // Reload game tree
      if (kifu) {
        player = new SgfPlayer(kifu.sgf_data, boardSize);
        mergeReviewsIntoPlayer();
        updatePlayerState();
      }

      const M = getM();
      if (M) {
        M.toast({ html: '添削を削除しました', classes: 'green darken-1' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red' });
      }
    }
  }

  function handleReturnToMainPath() {
    if (!player) return;

    const state = player.getCurrentState();
    let current = state.node;
    let targetNode: SgfNode | null = null;

    // Traverse up to find the first node that is not a variation (part of the primary path)
    while (current) {
      if (!(current as any).is_variation) {
        targetNode = current;
        break;
      }
      current = current.parent;
    }

    if (targetNode) {
      // Rebuild the primary path history completely
      player.initGame();
      
      // Find the target node inside the rebuilt primary path history
      const idx = player.history.findIndex(h => h.node === targetNode);
      if (idx !== -1) {
        player.goTo(idx);
      }
      updatePlayerState();

      const M = getM();
      if (M) {
        M.toast({ html: '本線（オリジナルの棋譜）に戻りました', classes: 'grey darken-3' });
      }
    }
  }


  // Clean up timers
  onDestroy(() => {
    stopAutoplay();
  });

  onMount(() => {
    if (auth.username) {
      reviewerName = auth.username;
    }
    loadKifu();
  });
</script>

<div class="row kifu-detail-root">
  <!-- Header Navigation（PC表示・スマホでは非表示） -->
  <div class="col s12 d-flex align-center justify-between kifu-header-nav" style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.2rem; flex-wrap: wrap; gap: 12px; border-bottom: 2px solid var(--wc-text); padding-bottom: 12px; position: relative; z-index: 10;">
    <div style="display: flex; align-items: center; flex-wrap: wrap; gap: 16px; position: relative;">
      <button class="em-collage-tag-pastel em-float-badge" onclick={onBack} style="cursor: pointer; display: inline-flex; align-items: center; gap: 4px; font-family: 'JetBrains Mono', sans-serif; font-size: 0.68rem; text-transform: uppercase; font-weight: bold; border: 1.5px solid var(--wc-text) !important; box-shadow: 2px 2px 0px var(--wc-text);">
        <i class="material-icons" style="font-size: 0.85rem; vertical-align: middle;">arrow_back</i>Back
      </button>
      {#if kifu}
        <h1 class="em-newspaper-headline" style="margin: 0; font-size: 1.6rem; font-family: 'Shippori Mincho B1', serif; font-weight: 700; color: var(--wc-text); line-height: 1.2;">
          {kifu.title}
        </h1>
      {/if}
    </div>
    {#if kifu && isOwner}
      <button class="share-settings-btn nm-btn em-pulse-button" onclick={() => showShareDialog = true}>
        <i class="material-icons" style="font-size: 0.95rem; vertical-align: middle; margin-right: 4px;">share</i>Share settings
      </button>
    {/if}
  </div>

  {#if loading}
    <div class="col s12 center-align" style="margin-top: 5rem;">
      <div class="preloader-wrapper big active">
        <div class="spinner-layer spinner-brown-only">
          <div class="circle-clipper left"><div class="circle"></div></div>
          <div class="gap-patch"><div class="circle"></div></div>
          <div class="circle-clipper right"><div class="circle"></div></div>
        </div>
      </div>
      <p class="grey-text">棋譜を構築中...</p>
    </div>
  {:else if error}
    <div class="col s12">
      <div class="card-panel red lighten-4 red-text text-darken-4">
        <i class="material-icons left">error</i>
        エラーが発生しました: {error}
      </div>
    </div>
  {:else}
    <!-- Main UI Grid -->
    <!-- Left Column: Go Board & Controls -->
    <div class="col s12 l7 center-align kifu-board-column" style="margin-bottom: 2rem; padding-bottom: 3.5rem; position: relative;">
      <div class="board-wrapper {isViewingVariation ? 'viewing-variation' : ''}" style="position: relative; display: inline-block;">
        {#if isViewingVariation}
          <div class="wc-variation-badge animate-fade-in" style="position: absolute; top: 15px; left: 15px; z-index: 10;">
            <i class="material-icons" style="font-size: 0.9rem; vertical-align: middle;">call_split</i>
            <span>指導手順: {activeReviewer ? `${activeReviewer} さん` : 'あなた'}</span>
          </div>
        {/if}

        <!-- Huge Vogue Background Deco Text overlapping under board -->
        <div style="position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%) rotate(-4deg); opacity: 0.08; font-size: 8rem; font-weight: 900; font-family: 'Cormorant Garamond', serif; text-transform: uppercase; letter-spacing: 0.15em; color: var(--wc-text); pointer-events: none; white-space: nowrap; z-index: 0; user-select: none;">
          {#if kifu}{kifu.result || 'THE GAME'}{:else}THE GAME{/if}
        </div>

        <div style="position: relative; z-index: 2;">
          <Board
            board={boardState}
            size={boardSize}
            lastMove={lastMove}
            interactive={reviewMode}
            turnColor={currentTurn}
            candidates={aiCandidates}
            previewStones={aiPreviewStones}
            influenceMap={influenceMap}
            showInfluence={showInfluenceMap}
            deadStones={deadStones}
            showDeadStones={showDeadStones}
            onIntersectionClick={handleIntersectionClick}
            onCandidateHover={handleCandidateHover}
            onCandidateClick={handleCandidateClick}
          />
        </div>
      </div>

      <!-- Playback Controls -->
      <div class="em-portfolio-section" style="margin-top: 2rem; border-color: var(--wc-text) !important; padding: 28px 20px 20px 20px !important;">
        <!-- Overlapping Badge -->
        <span class="em-collage-tag" style="position: absolute; top: -14px; left: 16px; z-index: 10; font-size: 0.72rem; transform: rotate(-1deg);">
          Playback Controls — Navigation
        </span>

        <div class="card-content" style="padding: 12px 0 0 0;">
          <!-- Slider -->
          <div class="range-field d-flex align-center" style="display: flex; align-items: center; margin-bottom: 0.75rem;">
            <span style="font-weight: 700; min-width: 70px; color: var(--wc-text); font-size: 0.88rem; font-family: 'JetBrains Mono', monospace; background: var(--wc-surface-alt); border: 1.5px solid var(--wc-text); padding: 2px 8px; text-align: center; box-shadow: 2px 2px 0px var(--wc-text); transform: rotate(-0.5deg);">{currentIndex} / {maxIndex}手</span>
            <input
              type="range"
              min="0"
              max={maxIndex}
              value={currentIndex}
              oninput={handleSliderChange}
              style="margin: 0 15px; flex-grow: 1;"
            />
          </div>


          <!-- Buttons Row -->
          <div class="buttons-row" style="display: flex; justify-content: center; gap: 8px; flex-wrap: wrap; margin-top: 14px; align-items: center;">
            <button type="button" class="nm-btn-flat" onclick={goFirst} title="最初へ" aria-label="最初の手へ移動">
              <i class="material-icons" aria-hidden="true" style="font-size: 1.2rem; color: var(--wc-text);">first_page</i>
            </button>
            <button type="button" class="nm-btn-flat" onclick={() => stepBack(10)} title="10手戻る" aria-label="10手戻る">
              <i class="material-icons" aria-hidden="true" style="font-size: 1.2rem; color: var(--wc-text);">fast_rewind</i>
            </button>
            <button type="button" class="nm-btn-flat" onclick={() => stepBack(1)} title="1手戻る" aria-label="1手戻る">
              <i class="material-icons" aria-hidden="true" style="font-size: 1.2rem; color: var(--wc-text);">navigate_before</i>
            </button>
            <button type="button" class="nm-btn-primary em-pulse-button" onclick={toggleAutoplay} title={isAutoplay ? '一時停止' : '自動再生'} aria-label={isAutoplay ? '自動再生を一時停止する' : '自動再生を開始する'}>
              <i class="material-icons" aria-hidden="true" style="font-size: 1.25rem;">{isAutoplay ? 'pause' : 'play_arrow'}</i>
            </button>
            <button type="button" class="nm-btn-flat" onclick={() => stepForward(1)} title="1手進む" aria-label="1手進む">
              <i class="material-icons" aria-hidden="true" style="font-size: 1.2rem; color: var(--wc-text);">navigate_next</i>
            </button>
            <button type="button" class="nm-btn-flat" onclick={() => stepForward(10)} title="10手進む" aria-label="10手進む">
              <i class="material-icons" aria-hidden="true" style="font-size: 1.2rem; color: var(--wc-text);">fast_forward</i>
            </button>
            <button type="button" class="nm-btn-flat" onclick={goLast} title="最後へ" aria-label="最後の手へ移動">
              <i class="material-icons" aria-hidden="true" style="font-size: 1.2rem; color: var(--wc-text);">last_page</i>
            </button>
            <!-- 表示オプションアイコンボタン -->
            <button
              type="button"
              class="nm-btn-flat settings-icon-btn {showSettingsModal ? 'active' : ''}"
              onclick={() => showSettingsModal = true}
              title="表示オプション"
              aria-label="表示オプションを開く"
            >
              <i class="material-icons" aria-hidden="true" style="font-size: 1.2rem; color: var(--wc-text);">tune</i>
            </button>
          </div>

          {#if isViewingVariation}
            <div class="animate-fade-in" style="margin-top: 16px; display: flex; justify-content: center;">
              <button type="button" class="nm-btn" onclick={handleReturnToMainPath} style="border-radius: 0px !important; border: 1px solid var(--wc-text) !important; background: var(--wc-surface) !important; font-weight: 600; display: inline-flex; align-items: center; gap: 6px; padding: 6px 16px;">
                <i class="material-icons" aria-hidden="true" style="font-size: 1.15rem;">assignment_return</i>
                本線（元の棋譜）に戻る
              </button>
            </div>
          {/if}
        </div>
      </div>
    </div>

    <!-- Right Column: Game Info, Comments & Variations -->
    <div class="col s12 l5 text-left kifu-right-column" style="text-align: left; position: relative;">
      <!-- Game Info Card (Collapsible) -->
      {#if kifu}
      <div class="em-vogue-editorial-section game-info-card hoverable" style="margin-top: 1.5rem; margin-bottom: 2rem; border-bottom: 1.5px solid var(--wc-border); padding: 24px 0 20px 0 !important; position: relative;">
        <!-- Card Header Toggle (Click to Toggle) - Slanted Collage Tag overlapping top border -->
        <button type="button" class="card-header-toggle cursor-pointer" onclick={() => isGameInfoExpanded = !isGameInfoExpanded} aria-expanded={isGameInfoExpanded} aria-controls="game-info-details" style="width: 100%; border: none; background: none; display: flex; align-items: center; justify-content: space-between; cursor: pointer; user-select: none; text-align: left; padding: 0;">
          <span class="em-collage-tag" style="position: absolute; top: -14px; left: 0; z-index: 10; font-size: 0.72rem; box-shadow: 2px 2px 0 var(--wc-text);">
            Info — Record Specs
          </span>
          <span style="font-weight: 700; font-size: 0.95rem; color: var(--wc-text); font-family: 'Shippori Mincho B1', serif; padding-top: 6px;">
            ● {kifu.black_player || 'Unknown'} vs ○ {kifu.white_player || 'Unknown'}
          </span>
          <div class="nm-btn-flat" style="width: 28px; height: 28px; min-width: 28px; padding: 0; border: 1px solid var(--wc-text) !important; border-radius: 0 !important; background: var(--wc-surface) !important; display: flex; align-items: center; justify-content: center;">
            <i class="material-icons" aria-hidden="true" style="font-size: 1.1rem; transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1); transform: {isGameInfoExpanded ? 'rotate(180deg)' : 'rotate(0deg)'}; color: var(--wc-text);">keyboard_arrow_down</i>
          </div>
        </button>

        <!-- Expanded details -->
        {#if isGameInfoExpanded}
          <div id="game-info-details" class="card-content" style="padding: 16px 0 0 0; border-top: 1.5px solid var(--wc-text); margin-top: 14px; transition: all 0.3s ease;">
            <table class="em-table">
              <tbody>
                <tr>
                  <th style="font-weight: 700; color: var(--wc-text); border-bottom-color: var(--wc-text);">● 黒番 (PB)</th>
                  <td style="border-bottom-color: var(--wc-text);">
                    <span style="font-weight: 700; color: var(--wc-text);">{kifu.black_player || '不明'}</span>
                    {#if kifu.black_rank}
                      <span class="wc-tag" style="margin-left: 6px; font-size: 0.72rem; padding: 1px 6px; border: 1px solid var(--wc-text); border-radius: 0; background: var(--wc-surface-alt);">{kifu.black_rank}</span>
                    {/if}
                    <div style="font-size: 0.8rem; color: var(--wc-text-muted); margin-top: 2px;">
                      アゲハマ: <strong style="font-family: 'JetBrains Mono', monospace; font-size: 0.85rem; color: var(--wc-text);">{captives.W}</strong> 子
                    </div>
                  </td>
                </tr>
                <tr>
                  <th style="font-weight: 700; color: var(--wc-text); border-bottom-color: var(--wc-text);">○ 白番 (PW)</th>
                  <td style="border-bottom-color: var(--wc-text);">
                    <span style="font-weight: 700; color: var(--wc-text);">{kifu.white_player || '不明'}</span>
                    {#if kifu.white_rank}
                      <span class="wc-tag" style="margin-left: 6px; font-size: 0.72rem; padding: 1px 6px; border: 1px solid var(--wc-text); border-radius: 0; background: var(--wc-surface-alt);">{kifu.white_rank}</span>
                    {/if}
                    <div style="font-size: 0.8rem; color: var(--wc-text-muted); margin-top: 2px;">
                      アゲハマ: <strong style="font-family: 'JetBrains Mono', monospace; font-size: 0.85rem; color: var(--wc-text);">{captives.B}</strong> 子
                    </div>
                  </td>
                </tr>
                <tr>
                  <th style="font-weight: 700; color: var(--wc-text); border-bottom-color: var(--wc-text);">コミ / 置き石</th>
                  <td style="font-family: 'JetBrains Mono', monospace; border-bottom-color: var(--wc-text);">コミ: {kifu.komi}目 / {kifu.handicap}子</td>
                </tr>
                <tr>
                  <th style="font-weight: 700; color: var(--wc-text); border-bottom-color: var(--wc-text);">対局結果</th>
                  <td style="font-weight: 700; color: var(--wc-accent); border-bottom-color: var(--wc-text);">{kifu.result || '対局中'}</td>
                </tr>
                <tr>
                  <th style="font-weight: 700; color: var(--wc-text); border-bottom-color: var(--wc-text);">対局日</th>
                  <td style="border-bottom-color: var(--wc-text);">{kifu.game_date || '未登録'}</td>
                </tr>
              </tbody>
            </table>
          </div>
        {/if}
      </div>
      {/if}

      <!-- AI Analysis Chart & Switch -->
      {#if hasAiAnalysis && aiData.length > 0}
        <div class="em-vogue-editorial-section animate-fade-in" style="margin-top: 1.5rem; margin-bottom: 2rem; border-bottom: 1.5px solid var(--wc-border); padding: 24px 0 20px 0 !important; position: relative;">
          <!-- Slanted Collage Tag -->
          <span class="em-collage-tag-pastel" style="position: absolute; top: -14px; left: 0; z-index: 10; font-size: 0.72rem; box-shadow: 2px 2px 0 var(--wc-text); background: var(--wc-accent-warm) !important; color: var(--wc-text) !important;">
            AI Analysis — 分析グラフ
          </span>
          
          <div class="card-content" style="padding: 16px 0 0 0;">
            <!-- Toggle Display Mode -->
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; flex-wrap: wrap; gap: 8px;">
              <div style="display: flex; gap: 6px;">
                <button 
                  class="nm-btn-flat {graphMode === 'winrate' ? 'active-tab' : ''}" 
                  onclick={() => graphMode = 'winrate'}
                  style="padding: 3px 8px; font-size: 0.75rem; font-weight: bold; border: 1.5px solid var(--wc-text) !important; border-radius: 0; background: {graphMode === 'winrate' ? 'var(--wc-text)' : 'var(--wc-surface)'} !important; color: {graphMode === 'winrate' ? 'var(--wc-surface)' : 'var(--wc-text)'} !important; cursor: pointer;"
                >
                  勝率
                </button>
                <button 
                  class="nm-btn-flat {graphMode === 'score' ? 'active-tab' : ''}" 
                  onclick={() => graphMode = 'score'}
                  style="padding: 3px 8px; font-size: 0.75rem; font-weight: bold; border: 1.5px solid var(--wc-text) !important; border-radius: 0; background: {graphMode === 'score' ? 'var(--wc-text)' : 'var(--wc-surface)'} !important; color: {graphMode === 'score' ? 'var(--wc-surface)' : 'var(--wc-text)'} !important; cursor: pointer;"
                >
                  目数差
                </button>
              </div>
              
              <!-- Toggle show candidates on board removed (moved to display options accordion) -->
            </div>

            <!-- Graph Container -->
            <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
            <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
            <div 
              class="ai-graph-container" 
              style="position: relative; height: 120px; border: 2.5px solid var(--wc-text); background: var(--wc-surface-alt); padding: 8px; box-sizing: border-box; box-shadow: 3px 3px 0px var(--wc-text); cursor: crosshair;"
              onmousemove={handleGraphMouseMove}
              onmouseleave={handleGraphMouseLeave}
              onclick={handleGraphClick}
              onkeydown={handleGraphKeyDown}
              tabindex="0"
              role="application"
              aria-label="勝率と目数差の推移グラフ。左右の矢印キーで手数を進退できます。"
            >
              <!-- Render SVG Graph -->
              <svg 
                viewBox="0 0 {graphWidth} {graphHeight}" 
                width="100%" 
                height="100%" 
                preserveAspectRatio="none"
                style="display: block; overflow: visible; pointer-events: none;"
              >
                <!-- Draw zero-line for score lead -->
                {#if graphMode === 'score'}
                  {@const scores = aiData.map(d => d.scoreLead)}
                  {@const maxScore = Math.max(10, ...scores)}
                  {@const minScore = Math.min(-10, ...scores)}
                  {@const scoreRange = maxScore - minScore}
                  {@const zeroY = graphHeight - ((0 - minScore) / scoreRange) * graphHeight}
                  {#if zeroY >= 0 && zeroY <= graphHeight}
                    <line x1="0" y1={zeroY} x2={graphWidth} y2={zeroY} stroke="#aaa" stroke-width="1.2" stroke-dasharray="3,3" />
                  {/if}
                {/if}
                
                <!-- Draw winrate 50% line -->
                {#if graphMode === 'winrate'}
                  <line x1="0" y1={graphHeight / 2} x2={graphWidth} y2={graphHeight / 2} stroke="#aaa" stroke-width="1.2" stroke-dasharray="3,3" />
                {/if}

                <!-- Grid Lines (vertical, every 50 moves) -->
                {#each Array(Math.ceil(maxIndex / 50)) as _, i}
                  {@const idx = (i + 1) * 50}
                  {@const x = (idx / maxIndex) * graphWidth}
                  {#if x < graphWidth}
                    <line x1={x} y1="0" x2={x} y2={graphHeight} stroke="rgba(0,0,0,0.06)" stroke-width="1" />
                  {/if}
                {/each}

                <!-- Draw Points and Path -->
                <!-- Fill Area under graph (Winrate) -->
                {#if graphMode === 'winrate' && graphPoints.length > 0}
                  <polygon 
                    points="0,{graphHeight} {graphPointsPath} {graphWidth},{graphHeight}" 
                    fill="rgba(33, 150, 243, 0.08)"
                  />
                {/if}

                <!-- Line -->
                {#if graphPoints.length > 0}
                  <polyline 
                    points={graphPointsPath} 
                    fill="none" 
                    stroke="var(--wc-text)" 
                    stroke-width="1.8" 
                  />
                {/if}

                <!-- Current Move Indicator (vertical accent line) -->
                {#if currentX >= 0 && currentX <= graphWidth}
                  <line 
                    x1={currentX} 
                    y1="0" 
                    x2={currentX} 
                    y2={graphHeight} 
                    stroke="var(--wc-accent)" 
                    stroke-width="1.8" 
                  />
                  {@const curPt = graphPoints.find(p => p.d.moveNumber === currentIndex)}
                  {#if curPt}
                    <circle 
                      cx={currentX} 
                      cy={curPt.y} 
                      r="4" 
                      fill="var(--wc-accent)"
                      stroke="var(--wc-surface)"
                      stroke-width="1.5"
                    />
                  {/if}
                {/if}

                <!-- Hover Indicator -->
                {#if graphHoverIndex !== null}
                  {@const hoverX = (graphHoverIndex / maxIndex) * graphWidth}
                  <line 
                    x1={hoverX} 
                    y1="0" 
                    x2={hoverX} 
                    y2={graphHeight} 
                    stroke="var(--wc-text-muted)" 
                    stroke-width="1" 
                    stroke-dasharray="2,2" 
                  />
                {/if}
              </svg>
            </div>

            <!-- Graph Hover Tooltip / Status -->
            <div style="margin-top: 14px; font-family: 'JetBrains Mono', monospace; font-size: 0.78rem; color: var(--wc-text); display: flex; justify-content: space-between; align-items: center; border: 1.5px solid var(--wc-text); padding: 6px 12px; background: var(--wc-surface-alt); box-shadow: 2px 2px 0px var(--wc-text);">
              {#if graphHoverIndex !== null}
                {@const hoverD = aiData.find(d => d.moveNumber === graphHoverIndex)}
                <span style="font-weight: 700;">{graphHoverIndex}手目 ({graphHoverIndex % 2 === 0 ? '○白' : '●黒'})</span>
                {#if hoverD}
                  <span>勝率: <strong>{hoverD.winrate.toFixed(1)}%</strong></span>
                  <span>目数差: <strong>{hoverD.scoreLead > 0 ? `黒+${hoverD.scoreLead.toFixed(1)}` : `白+${Math.abs(hoverD.scoreLead).toFixed(1)}`}</strong>目</span>
                {:else}
                  <span style="opacity: 0.5;">データなし</span>
                {/if}
              {:else}
                {@const curD = aiData.find(d => d.moveNumber === currentIndex)}
                <span style="font-weight: 700;">現在: {currentIndex}手目 ({currentIndex % 2 === 0 ? '○白' : '●黒'})</span>
                {#if curD}
                  <span>勝率: <strong>{curD.winrate.toFixed(1)}%</strong></span>
                  <span>目数差: <strong>{curD.scoreLead > 0 ? `黒+${curD.scoreLead.toFixed(1)}` : `白+${Math.abs(curD.scoreLead).toFixed(1)}`}</strong>目</span>
                {:else}
                  <span style="opacity: 0.5;">AI分析なし</span>
                {/if}
              {/if}
            </div>
            
            <!-- Candidates List Detail -->
            {#if aiCandidates && aiCandidates.length > 0}
              <div style="margin-top: 14px; border: 1.5px solid var(--wc-text); padding: 10px; background: var(--wc-surface);">
                <div style="font-size: 0.8rem; font-weight: bold; margin-bottom: 6px; border-bottom: 1px solid var(--wc-border); padding-bottom: 4px;">AI候補手リスト</div>
                <div style="display: flex; flex-direction: column; gap: 6px;">
                  {#each aiCandidates as cand}
                    <button 
                      type="button"
                      class="candidate-row"
                      onmouseover={() => !isMobileDevice && handleCandidateHover(cand)}
                      onmouseleave={() => !isMobileDevice && handleCandidateHover(null)}
                      onfocus={() => !isMobileDevice && handleCandidateHover(cand)}
                      onblur={() => !isMobileDevice && handleCandidateHover(null)}
                      onclick={() => handleCandidateClick(cand)}
                      style="display: flex; align-items: center; justify-content: space-between; font-size: 0.78rem; padding: 4px 6px; cursor: pointer; border: 1px solid transparent; transition: background 0.15s; background: none; width: 100%; border-radius: 0; text-align: left; color: inherit; font-family: inherit;"
                      class:hovered={activeHoveredCandidate === cand}
                    >
                      <div style="display: flex; align-items: center; gap: 8px;">
                        <span 
                          style="display: inline-block; width: 14px; height: 14px; border-radius: 50%; border: 1px solid var(--wc-text); background: {getCandidateColor(cand)};"
                        ></span>
                        <span style="font-family: 'JetBrains Mono', monospace; font-weight: bold;">
                          {cand.isBest ? '最善手' : `候補 ${cand.rank}`} ({cand.coords})
                        </span>
                      </div>
                      <div style="font-family: 'JetBrains Mono', monospace; opacity: 0.85;">
                        {#if cand.loss > 0}
                          <span style="color: var(--wc-accent); font-weight: bold;">-{cand.loss.toFixed(1)}目</span>
                        {:else}
                          <span style="color: #4CAF50; font-weight: bold;">最善</span>
                        {/if}
                      </div>
                    </button>
                  {/each}
                </div>
                <div style="font-size: 0.65rem; color: var(--wc-text-muted); margin-top: 6px; text-align: right;">
                  {#if isMobileDevice}
                    ※候補手をタップすると変化図をプレビュー、もう一度タップすると変化図へ遷移
                  {:else}
                    ※候補手をホバーすると変化図をプレビュー、クリックすると変化図へ遷移
                  {/if}
                </div>
              </div>
            {/if}
          </div>
        </div>
      {/if}

      <!-- Branches & Variations Card -->
      {#if alternativeBranches.length > 0}
        <div class="em-vogue-editorial-section animate-fade-in" style="margin-top: 1.5rem; margin-bottom: 2rem; border-bottom: 1.5px solid var(--wc-border); padding: 24px 0 20px 0 !important; position: relative;">
          <!-- Slanted Collage Tag -->
          <span class="em-collage-tag-pastel" style="position: absolute; top: -14px; left: 0; z-index: 10; font-size: 0.72rem; box-shadow: 2px 2px 0px var(--wc-text); background: var(--wc-accent-warm) !important; color: var(--wc-text) !important;">
            Variations — 変化図
          </span>
          <div class="card-content" style="padding: 16px 0 0 0;">
            <div style="display: flex; gap: 10px; flex-wrap: wrap;">
              <!-- Main branch return button if not on primary branch -->
              {#if isViewingVariation}
                <button type="button" class="share-settings-btn" onclick={handleReturnToMainPath} style="display: inline-flex; align-items: center; gap: 4px; font-weight: bold; background: var(--wc-surface) !important; border: 1.5px solid var(--wc-text) !important;">
                  <i class="material-icons" aria-hidden="true" style="font-size: 1.05rem; vertical-align: middle;">assignment_return</i>
                  本線に戻る
                </button>
              {/if}
              <!-- Display other branch buttons -->
              {#each alternativeBranches as branch}
                <button type="button" class="share-settings-btn" onclick={() => selectBranch(branch.originalIndex)} style="background: var(--wc-surface-alt) !important; border: 1.5px solid var(--wc-text) !important; display: inline-flex; align-items: center; gap: 4px;">
                  <i class="material-icons" aria-hidden="true" style="font-size: 1.05rem; vertical-align: middle;">call_split</i>
                  {branch.label}
                </button>
              {/each}
            </div>
          </div>
        </div>
      {/if}

      <!-- Comments / Review Card -->
      <div class="em-vogue-editorial-section" style="min-height: 180px; display: flex; flex-direction: column; margin-bottom: 2rem; border-bottom: 1.5px solid var(--wc-border); padding: 24px 0 20px 0 !important; position: relative;">
        <div class="card-content" style="padding: 16px 0 0 0; flex-grow: 1; position: relative;">
          <!-- Overlapping Title with Collage style -->
          <span class="em-collage-tag-pastel" style="position: absolute; top: -38px; left: 0; z-index: 10; font-size: 0.72rem; box-shadow: 2px 2px 0px var(--wc-text);">
            Reviews (第 {currentIndex} 手)
          </span>

          {#if comments.length === 0}
            <div class="center-align" style="padding: 2.5rem 0; opacity: 0.5;">
              <i class="material-icons" aria-hidden="true" style="font-size: 2.6rem; color: var(--wc-text-muted);">chat_bubble_outline</i>
              <p style="margin: 8px 0 0 0; font-size: 0.88rem; color: var(--wc-text-muted); font-family: 'DM Sans', sans-serif;">この手に対するコメントはありません</p>
            </div>
          {:else}
            <div class="comments-list" style="max-height: 250px; overflow-y: auto; padding-right: 4px;">
              {#each comments as comment}
                <div class="comment-item" style="border-bottom: 1.5px dashed var(--wc-border); padding: 14px 0; display: flex; justify-content: space-between; align-items: flex-start;">
                  <div style="flex-grow: 1; text-align: left;">
                    <span class="wc-tag" style="font-weight: 700; margin-bottom: 8px; padding: 2px 10px; font-size: 0.7rem; letter-spacing: 0.05em; border: 1px solid var(--wc-text); border-radius: 0; background: var(--wc-surface-alt); color: var(--wc-text);">
                      {comment.author}
                    </span>
                    {#if comment.isFromVariationRoot}
                      <span class="wc-tag animate-fade-in" style="margin-left: 6px; font-weight: 700; margin-bottom: 8px; padding: 2px 10px; font-size: 0.7rem; letter-spacing: 0.05em; border: 1.5px solid var(--wc-text); border-radius: 0; background: var(--wc-surface); color: var(--wc-accent); box-shadow: 1.5px 1.5px 0px var(--wc-text); display: inline-block;">
                        変化図の開始コメント
                      </span>
                    {/if}
                    <p style="margin: 0; padding-left: 2px; font-size: 0.92rem; white-space: pre-wrap; color: var(--wc-text); line-height: 1.6; font-family: 'DM Sans', 'Noto Sans JP', sans-serif;">{comment.text}</p>
                  </div>
                  {#if comment.reviewId && !isPublic && (isOwner || auth.username === comment.reviewerName)}
                    <div style="display: flex; gap: 6px; margin-left: 12px;">
                      <button type="button" class="nm-btn-flat" onclick={() => handleEditReview(comment.reviewId || '', comment.reviewerName || '', comment.text)} title="指導を編集" aria-label="指導を編集" style="width: 28px; height: 28px; min-width: 28px; padding: 0; border: 1px solid var(--wc-text) !important; border-radius: 0 !important; background: var(--wc-surface) !important; display: flex; align-items: center; justify-content: center;">
                        <i class="material-icons" aria-hidden="true" style="font-size: 1rem; color: var(--wc-text);">edit</i>
                      </button>
                      <button type="button" class="nm-btn-flat" onclick={() => handleDeleteReview(comment.reviewId || '')} title="指導を削除" aria-label="指導を削除" style="width: 28px; height: 28px; min-width: 28px; padding: 0; border: 1px solid var(--wc-text) !important; border-radius: 0 !important; background: var(--wc-surface) !important; display: flex; align-items: center; justify-content: center;">
                        <i class="material-icons" aria-hidden="true" style="font-size: 1rem; color: #e53935;">delete</i>
                      </button>
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Mode Toggle & Edit Panel Footer -->
        <div class="card-action" style="border-top: 1.5px solid var(--wc-text); padding: 16px 0 0 0; margin-top: 16px; background: transparent;">
          {#if isPublic}
            <div class="wc-info-banner animate-fade-in" style="margin-bottom: 16px; background: rgba(37,53,48,0.05); border: 1.5px solid var(--wc-border); padding: 12px; font-size: 0.78rem; color: var(--wc-text-muted); display: flex; align-items: center; gap: 6px; box-shadow: 2px 2px 0px var(--wc-text); margin-top: 8px;">
              <i class="material-icons" style="font-size: 0.95rem; color: var(--wc-accent-warm); vertical-align: middle;">info_outline</i>
              <span>一般公開中の棋譜は添削できません。添削を追加・編集するには限定公開に変更してください。</span>
            </div>
          {/if}

          <div class="switch" style="margin-bottom: 12px;">
            <label style="font-weight: 500; display: inline-flex; align-items: center; gap: 8px; color: var(--wc-text); font-family: 'DM Sans', 'Noto Sans JP', sans-serif;">
              <span style="color: var(--wc-text-muted); font-size: 0.88rem;">通常再生</span>
              <input type="checkbox" bind:checked={reviewMode} disabled={isPublic}>
              <span class="lever" style="background-color: var(--wc-border); border: 1px solid var(--wc-text);"></span>
              <span style="color: var(--wc-text); font-weight: 700; font-size: 0.88rem;">添削モード (盤面入力)</span>
            </label>
          </div>

          {#if reviewMode}
            <div class="review-edit-panel animate-fade-in" style="margin-top: 12px; border-top: 1.5px dashed var(--wc-text); padding-top: 16px;">
              <p style="font-size: 0.78rem; margin: 0 0 16px 0; color: var(--wc-text-muted); display: inline-flex; align-items: center; gap: 6px; font-family: 'DM Sans', 'Noto Sans JP', sans-serif; line-height: 1.45;">
                <i class="material-icons" style="font-size: 0.95rem; color: var(--wc-accent-warm);">info_outline</i>
                盤面をクリックして石を置くと変化図を作成できます。解説コメントを入力してください。
              </p>
              
              <div class="row" style="margin-bottom: 0; display: flex; flex-direction: column; gap: 12px;">
                <div class="input-field col s12" style="margin-top: 0; margin-bottom: 0;">
                  <input id="reviewer_name" type="text" class="nm-input" bind:value={reviewerName} placeholder="添削者名を入力" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
                  <label for="reviewer_name" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">添削者 (Reviewer)</label>
                </div>
                <div class="input-field col s12" style="margin-top: 4px; margin-bottom: 0;">
                  <input id="review_comment" type="text" class="nm-input" bind:value={reviewComment} placeholder="指導・変化図の解説を入力" style="margin-bottom: 0; border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; background: var(--wc-surface) !important;" />
                  <label for="review_comment" class="active" style="transform: translateY(-12px) scale(0.8); left: 0.75rem; color: var(--wc-text); font-weight: 600;">コメント (Comment)</label>
                </div>
                <div class="col s12 right-align" style="display: flex; justify-content: flex-end; gap: 10px; margin-top: 10px;">
                  {#if editingReviewId}
                    <button class="nm-btn-flat" style="border-radius: 0 !important;" onclick={handleCancelEdit} disabled={isAddingReview}>
                      キャンセル
                    </button>
                    <button class="nm-btn-primary" style="border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; box-shadow: 2px 2px 0px var(--wc-text) !important;" disabled={!reviewerName || !reviewComment || isAddingReview} onclick={handleSaveReview}>
                      <i class="material-icons" style="font-size: 1.1rem; vertical-align: middle; margin-right: 4px;">save</i>添削を更新
                    </button>
                  {:else}
                    <button class="nm-btn-primary" style="border-radius: 0 !important; border: 1.5px solid var(--wc-text) !important; box-shadow: 2px 2px 0px var(--wc-text) !important;" disabled={!reviewerName || !reviewComment || isAddingReview} onclick={handleSaveReview}>
                      <i class="material-icons" style="font-size: 1.1rem; vertical-align: middle; margin-right: 4px;">save</i>添削を保存
                    </button>
                  {/if}
                </div>
              </div>
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}

  <!-- スマホ専用ボトムナビバー（PCでは非表示） -->
  <div class="kifu-bottom-nav">
    <button class="kifu-bottom-nav-back" onclick={onBack}>
      <i class="material-icons" style="font-size: 1.1rem;">arrow_back</i>
      <span>Back</span>
    </button>
    {#if kifu}
      <span class="kifu-bottom-nav-title">{kifu.title}</span>
    {/if}
    {#if kifu && isOwner}
      <button class="kifu-bottom-nav-share" onclick={() => showShareDialog = true}>
        <i class="material-icons" style="font-size: 1.1rem;">share</i>
      </button>
    {/if}
  </div>
</div>

{#if showShareDialog && kifu}
  <ShareDialog 
    kifu={kifu} 
    currentPlayIndex={currentIndex}
    onClose={() => showShareDialog = false} 
    onUpdate={(updatedKifu: any) => {
      if (kifu) {
        kifu = { ...kifu, ...updatedKifu } as KifuDetailData;
      }
    }} 
  />
{/if}

<!-- 表示オプション ボトムシートモーダル -->
{#if showSettingsModal}
  <!-- Backdrop -->
  <div
    class="settings-backdrop"
    onclick={() => showSettingsModal = false}
    role="presentation"
  ></div>
  <!-- Sheet -->
  <div class="settings-sheet animate-slide-up" role="dialog" aria-modal="true" aria-label="表示オプション" tabindex="-1">
    <!-- Handle bar -->
    <div class="settings-sheet-handle"></div>
    <div class="settings-sheet-header">
      <span class="settings-sheet-title">
        <i class="material-icons" style="font-size: 1.1rem; vertical-align: middle; margin-right: 6px;">tune</i>
        表示オプション
      </span>
      <button type="button" class="settings-sheet-close" onclick={() => showSettingsModal = false} aria-label="閉じる">
        <i class="material-icons" style="font-size: 1.1rem;">close</i>
      </button>
    </div>
    <div class="settings-sheet-body">
      <!-- トグル行 -->
      <label class="settings-toggle-row">
        <span class="settings-toggle-label">
          <i class="material-icons settings-toggle-icon">blur_on</i>
          勢力図 <span class="settings-toggle-sub">Influence Map</span>
        </span>
        <button
          type="button"
          class="settings-switch {showInfluenceMap ? 'on' : ''}"
          onclick={() => showInfluenceMap = !showInfluenceMap}
          role="switch"
          aria-checked={showInfluenceMap}
          aria-label="勢力図を表示"
        >
          <span class="settings-switch-thumb"></span>
        </button>
      </label>

      <label class="settings-toggle-row">
        <span class="settings-toggle-label">
          <i class="material-icons settings-toggle-icon">circle</i>
          死活判定 <span class="settings-toggle-sub">Dead Stones</span>
        </span>
        <button
          type="button"
          class="settings-switch {showDeadStones ? 'on' : ''}"
          onclick={() => showDeadStones = !showDeadStones}
          role="switch"
          aria-checked={showDeadStones}
          aria-label="死活判定を表示"
        >
          <span class="settings-switch-thumb"></span>
        </button>
      </label>

      <label class="settings-toggle-row">
        <span class="settings-toggle-label">
          <i class="material-icons settings-toggle-icon">auto_fix_high</i>
          AI候補手 <span class="settings-toggle-sub">AI Moves</span>
        </span>
        <button
          type="button"
          class="settings-switch {showAiAnalysis ? 'on' : ''}"
          onclick={() => { showAiAnalysis = !showAiAnalysis; updatePlayerState(); }}
          role="switch"
          aria-checked={showAiAnalysis}
          aria-label="AI候補手を表示"
        >
          <span class="settings-switch-thumb"></span>
        </button>
      </label>

      <!-- 区切り線 -->
      <div class="settings-divider"></div>

      <!-- SGF出力ボタン -->
      <button
        type="button"
        class="settings-action-btn"
        onclick={() => { handleExportPath(); showSettingsModal = false; }}
      >
        <i class="material-icons" style="font-size: 1.1rem;">file_download</i>
        現在の手順をSGF出力
      </button>
    </div>
  </div>
{/if}


  <style>
  .animate-fade-in {
    animation: fadeIn 0.25s ease-out;
  }
  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(5px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .controls-panel {
    border-radius: 8px;
    overflow: hidden;
  }
  .comments-list::-webkit-scrollbar {
    width: 6px;
  }
  .comments-list::-webkit-scrollbar-track {
    background: var(--wc-bg);
  }
  .comments-list::-webkit-scrollbar-thumb {
    background: var(--wc-mid);
    border-radius: 3px;
  }
  .comments-list {
    scrollbar-width: thin;
  }

  /* Playback controls buttons styling */
  .buttons-row button {
    width: 38px;
    height: 38px;
    min-width: 38px;
    padding: 0 !important;
    background-color: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    transition: transform 0.1s ease, background-color 0.1s ease, box-shadow 0.1s ease !important;
    border: 1.5px solid var(--wc-text) !important;
    box-shadow: 2px 2px 0px var(--wc-text) !important;
    display: inline-flex !important;
    align-items: center;
    justify-content: center;
    touch-action: manipulation; /* タップ遅延防止 */
    border-radius: 0px !important;
  }

  /* Hover micro-animations (PC/マウスホバー環境のみ) */
  @media (hover: hover) {
    .buttons-row button:hover {
      background-color: var(--wc-surface-alt) !important;
      color: var(--wc-accent) !important;
      transform: scale(1.08) translateY(-1px) !important;
      box-shadow: var(--nm-shadow-outset-sm-hover) !important;
    }
  }

  .buttons-row button:active {
    transform: scale(0.95) !important;
    box-shadow: var(--nm-shadow-inset) !important;
    background-color: var(--wc-surface-alt) !important;
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

  .em-portfolio-section {
    width: 100%;
    max-width: min(78vh, 720px);
    box-sizing: border-box;
  }

  /* Touch and Tablet responsive adjustments (拡大ボタン・全幅スライダー) */
  @media (pointer: coarse), only screen and (max-width: 1024px) {
    .range-field {
      flex-direction: column !important;
      align-items: center !important;
      gap: 12px !important;
    }
    .range-field input[type=range] {
      width: 100% !important;
      margin: 0 !important;
    }
    .range-field span {
      font-size: 1.05rem !important;
      padding: 4px 16px !important;
    }
    .buttons-row {
      display: flex !important;
      flex-wrap: nowrap !important;
      justify-content: center !important;
      gap: 6px !important;
    }
    .buttons-row button {
      width: 44px !important;
      height: 44px !important;
      line-height: 44px !important;
      min-width: 44px !important;
      padding: 0 !important;
      flex: 0 1 auto !important;
    }
    .buttons-row button i {
      font-size: 1.5rem !important;
      line-height: 44px !important;
    }
  }

  /* 極小スマホ画面 (width: 360px以下) でのさらなる微調整 */
  @media only screen and (max-width: 360px) {
    .buttons-row {
      gap: 4px !important;
    }
    .buttons-row button {
      width: 38px !important;
      height: 38px !important;
      line-height: 38px !important;
      min-width: 38px !important;
    }
    .buttons-row button i {
      font-size: 1.3rem !important;
      line-height: 38px !important;
    }
  }

  /* Mobile screen responsive adjustments (狭い画面での余白・碁盤枠線調整) */
  @media only screen and (max-width: 600px) {
    .kifu-board-column {
      padding-left: 6px !important;
      padding-right: 6px !important;
    }
    .board-wrapper {
      border-width: 2px !important;
      padding: 2px !important;
      border-radius: 6px !important;
      max-width: 100% !important;
    }
    .em-portfolio-section {
      padding: 20px 10px 14px 10px !important;
      margin-top: 1rem !important;
      max-width: 100% !important;
    }
    :global(.card-content) {
      padding: 12px !important;
    }
    :global(.card-action) {
      padding: 12px !important;
    }
    .review-edit-panel .row .input-field {
      margin-bottom: 5px !important;
    }
    h5 {
      font-size: 1.25rem !important;
    }
  }

  .board-wrapper.viewing-variation {
    border-color: var(--wc-accent-warm) !important;
    background-color: var(--wc-surface-alt) !important;
    box-shadow: 6px 6px 0px var(--wc-text) !important;
  }

  /* Washi Clay Variation Badge */
  .wc-variation-badge {
    background: var(--wc-surface-alt);
    border: 1px solid var(--wc-accent-warm);
    padding: 4px 12px;
    border-radius: var(--wc-radius-sm);
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--wc-accent-warm);
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  /* Share Settings Button custom hover and active animations */
  .share-settings-btn {
    border-radius: 0 !important;
    font-family: 'JetBrains Mono', sans-serif;
    font-size: 0.72rem;
    text-transform: uppercase;
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-surface) !important;
    box-shadow: 2.5px 2.5px 0px var(--wc-text) !important;
    font-weight: bold;
    padding: 4px 14px !important;
    color: var(--wc-text) !important;
    transition: transform 0.15s cubic-bezier(0.25, 0.46, 0.45, 0.94), box-shadow 0.15s cubic-bezier(0.25, 0.46, 0.45, 0.94), background-color 0.15s ease !important;
    touch-action: manipulation; /* タップ遅延防止 */
  }

  @media (hover: hover) {
    .share-settings-btn:hover {
      animation-play-state: paused !important; /* Pause pulse-btn animation on hover */
      transform: translate(-1.5px, -1.5px) scale(1.03) !important;
      box-shadow: 4px 4px 0px var(--wc-text) !important;
      background-color: var(--wc-surface-alt) !important;
    }
  }

  .share-settings-btn:active {
    transform: translate(1px, 1px) scale(0.97) !important;
    box-shadow: 1px 1px 0px var(--wc-text) !important;
  }

  @media (min-width: 993px) {
    .kifu-right-column {
      padding-left: 2rem !important;
      border-left: 1.5px dashed var(--wc-border);
    }
  }

  .candidate-row:hover, .candidate-row.hovered {
    background: var(--wc-surface-alt);
    border-color: var(--wc-text) !important;
  }
  /* ---- スマホ：ヘッダーナビ非表示 / ボトムバー表示 ---- */
  @media only screen and (max-width: 600px) {
    .kifu-header-nav {
      display: none !important;
    }
    .kifu-detail-root {
      margin-top: 0 !important;
    }
  }

  /* ---- PC：ボトムバー非表示 ---- */
  .kifu-bottom-nav {
    display: none;
  }
  @media only screen and (max-width: 600px) {
    .kifu-bottom-nav {
      display: flex;
      align-items: center;
      gap: 10px;
      position: fixed;
      bottom: 0;
      left: 0;
      right: 0;
      z-index: 200;
      background: var(--wc-surface);
      border-top: 2px solid var(--wc-text);
      box-shadow: 0 -3px 0 0 var(--wc-text);
      padding: 6px 12px;
      padding-bottom: max(6px, env(safe-area-inset-bottom));
      min-height: 48px;
    }
    .kifu-bottom-nav-back {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-family: 'JetBrains Mono', sans-serif;
      font-size: 0.68rem;
      font-weight: bold;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      background: var(--wc-surface-alt);
      border: 1.5px solid var(--wc-text);
      box-shadow: 2px 2px 0 var(--wc-text);
      padding: 4px 10px;
      color: var(--wc-text);
      cursor: pointer;
      flex-shrink: 0;
    }
    .kifu-bottom-nav-title {
      flex: 1;
      font-family: 'Shippori Mincho B1', serif;
      font-size: 0.78rem;
      font-weight: 700;
      color: var(--wc-text);
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
      text-align: center;
      opacity: 0.75;
    }
    .kifu-bottom-nav-share {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 36px;
      height: 36px;
      background: var(--wc-surface-alt);
      border: 1.5px solid var(--wc-text);
      box-shadow: 2px 2px 0 var(--wc-text);
      color: var(--wc-text);
      cursor: pointer;
      flex-shrink: 0;
    }
  }

  /* ---- 表示オプション モーダルシート ---- */
  :global(.settings-backdrop) {
    position: fixed;
    inset: 0;
    background: rgba(37, 53, 48, 0.35);
    z-index: 900;
    backdrop-filter: blur(2px);
  }

  :global(.settings-sheet) {
    position: fixed;
    left: 50%;
    bottom: 0;
    transform: translateX(-50%);
    width: min(480px, 100%);
    z-index: 901;
    background: var(--wc-surface);
    border: 2px solid var(--wc-text);
    border-bottom: none;
    box-shadow: 0 -4px 0 0 var(--wc-text);
    padding-bottom: max(16px, env(safe-area-inset-bottom));
    border-radius: 0;
  }

  @keyframes slideUp {
    from { transform: translateX(-50%) translateY(100%); }
    to   { transform: translateX(-50%) translateY(0); }
  }
  :global(.animate-slide-up) {
    animation: slideUp 0.22s cubic-bezier(0.25, 0.8, 0.25, 1) both;
  }

  :global(.settings-sheet-handle) {
    width: 36px;
    height: 4px;
    background: var(--wc-border);
    border-radius: 2px;
    margin: 10px auto 6px;
  }

  :global(.settings-sheet-header) {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 16px 10px;
    border-bottom: 1.5px solid var(--wc-text);
  }

  :global(.settings-sheet-title) {
    font-family: 'Shippori Mincho B1', serif;
    font-size: 0.9rem;
    font-weight: 700;
    color: var(--wc-text);
    display: inline-flex;
    align-items: center;
  }

  :global(.settings-sheet-close) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 30px;
    height: 30px;
    border: 1.5px solid var(--wc-text);
    background: var(--wc-surface-alt);
    box-shadow: 2px 2px 0 var(--wc-text);
    color: var(--wc-text);
    cursor: pointer;
  }

  :global(.settings-sheet-body) {
    padding: 8px 16px 4px;
  }

  :global(.settings-toggle-row) {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid var(--wc-border);
    cursor: pointer;
    user-select: none;
  }

  :global(.settings-toggle-label) {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    font-family: 'Shippori Mincho B1', serif;
    font-size: 0.88rem;
    font-weight: 600;
    color: var(--wc-text);
  }

  :global(.settings-toggle-icon) {
    font-size: 1.15rem;
    color: var(--wc-text);
    opacity: 0.7;
  }

  :global(.settings-toggle-sub) {
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.65rem;
    font-weight: 400;
    opacity: 0.5;
    margin-left: 4px;
  }

  /* トグルスイッチ */
  :global(.settings-switch) {
    position: relative;
    width: 42px;
    height: 24px;
    border: 1.5px solid var(--wc-text);
    background: var(--wc-surface-alt);
    box-shadow: 2px 2px 0 var(--wc-text);
    cursor: pointer;
    flex-shrink: 0;
    transition: background 0.15s ease;
    padding: 0;
  }
  :global(.settings-switch.on) {
    background: var(--wc-text);
  }
  :global(.settings-switch-thumb) {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 16px;
    height: 16px;
    background: var(--wc-text);
    transition: transform 0.15s ease, background 0.15s ease;
  }
  :global(.settings-switch.on .settings-switch-thumb) {
    transform: translateX(18px);
    background: var(--wc-surface);
  }

  :global(.settings-divider) {
    height: 1px;
    background: var(--wc-text);
    margin: 6px 0 10px;
    opacity: 0.2;
  }

  :global(.settings-action-btn) {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 10px 12px;
    background: var(--wc-surface-alt);
    border: 1.5px solid var(--wc-text);
    box-shadow: 2px 2px 0 var(--wc-text);
    font-family: 'JetBrains Mono', monospace;
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--wc-text);
    cursor: pointer;
    margin-bottom: 8px;
    text-align: left;
    letter-spacing: 0.03em;
  }

  /* ⚙ アイコンボタン アクティブ時 */
  .settings-icon-btn.active {
    background: var(--wc-text) !important;
    color: var(--wc-surface) !important;
  }
  .settings-icon-btn.active i {
    color: var(--wc-surface) !important;
  }
</style>
