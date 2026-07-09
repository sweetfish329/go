<script lang="ts">
  import { onMount } from 'svelte';
  import QRCode from 'qrcode';
  import { auth } from '../lib/auth.svelte';

  interface Kifu {
    id: string;
    title: string;
    is_private?: boolean;
    share_token?: string;
    black_player?: string;
    black_rank?: string;
    white_player?: string;
    white_rank?: string;
    result?: string;
    game_date?: string;
  }

  let { kifu, onClose, onUpdate } = $props<{
    kifu: Kifu;
    onClose: () => void;
    onUpdate: (updatedKifu: Kifu) => void;
  }>();

  let loading = $state(false);
  let qrCodeSvg = $state("");
  let copySuccess = $state(false);
  let isPrivate = $state(false);

  $effect(() => {
    isPrivate = kifu.is_private !== false;
  });

  const getM = () => (window as any).M;

  // Compute share URL: Use OIDC share token url if private, otherwise public archive url
  const shareUrl = $derived.by(() => {
    const origin = window.location.origin;
    if (isPrivate) {
      return kifu.share_token ? `${origin}/?share=${kifu.share_token}` : "";
    }
    return `${origin}/u/${auth.userId}/${kifu.id}`;
  });

  async function autoGenerateShareToken() {
    loading = true;
    try {
      const res = await fetch(`/api/kifus/${kifu.id}/share`, {
        method: 'POST',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          disable: false,
          expires_in_days: null
        })
      });
      const updated = await res.json();
      if (!res.ok) {
        throw new Error(updated.error || "トークンの自動生成に失敗しました。");
      }
      onUpdate(updated);
    } catch (err: any) {
      console.error("Failed to auto-generate share token:", err);
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    if (isPrivate && !kifu.share_token && !loading) {
      autoGenerateShareToken();
    }
  });

  // Automatically render and upload OGP image when shared dialog is mounted
  onMount(() => {
    // Delay slightly to ensure Board SVG is rendered in DOM
    setTimeout(generateAndUploadOgp, 500);
  });

  function generateAndUploadOgp(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Prioritize the hidden final state board so OGP is always generated from the finished game state
      const svgEl = (document.querySelector(".final-board-hidden svg") || document.querySelector(".go-board")) as SVGSVGElement;
      if (!svgEl) {
        console.warn("Go board SVG not found");
        resolve(); // Resolve silently if board is not visible
        return;
      }

      try {
        // Clone the SVG element to avoid modifying the visible DOM
        const svgClone = svgEl.cloneNode(true) as SVGSVGElement;
        
        // Remove filters from all elements in the SVG clone to prevent canvas rendering errors (like black shapes)
        const filteredElements = svgClone.querySelectorAll('[filter]');
        filteredElements.forEach(el => {
          el.removeAttribute('filter');
        });

        // Query and replace all CSS variables in the cloned SVG with solid fallback colors.
        // Since SVG loaded into <img> context cannot access document CSS variables, they resolve to black.
        const wcBoardValue = '#ebdac0'; // --wc-board actual color
        const wcTextValue = '#253530';  // --wc-text / --wc-border actual color
        const wcSurfaceValue = '#f4f6f5'; // --wc-surface actual color

        const replaceCssVars = (el: Element, attrName: string) => {
          const val = el.getAttribute(attrName);
          if (!val) return;
          let newVal = val;
          newVal = newVal.replace(/var\(--wc-board\)/g, wcBoardValue);
          newVal = newVal.replace(/var\(--wc-text\)/g, wcTextValue);
          newVal = newVal.replace(/var\(--wc-border\)/g, wcTextValue);
          newVal = newVal.replace(/var\(--wc-surface\)/g, wcSurfaceValue);
          el.setAttribute(attrName, newVal);
        };

        const allCloneElements = svgClone.querySelectorAll('*');
        allCloneElements.forEach(el => {
          replaceCssVars(el, 'fill');
          replaceCssVars(el, 'stroke');
        });

        // Replace complex SVG definitions and gradients with clean solid colors to guarantee 100% successful canvas drawing
        // 1. Board Background (Ash wood Milk Tea Beige)
        const boardBg = svgClone.querySelector('rect[fill="url(#boardWood)"]');
        if (boardBg) {
          boardBg.setAttribute('fill', '#ebdac0'); 
        }

        // 2. Black Stones (Solid Dark Sage / Charcoal)
        const blackStones = svgClone.querySelectorAll('circle[fill="url(#blackStoneGrad)"]');
        blackStones.forEach(stone => {
          stone.setAttribute('fill', '#253530'); 
        });

        // 3. White Stones (Solid White with a thin dark sage border for contrast)
        const whiteStones = svgClone.querySelectorAll('circle[fill="url(#whiteStoneGrad)"]');
        whiteStones.forEach(stone => {
          stone.setAttribute('fill', '#ffffff');
          stone.setAttribute('stroke', '#253530');
          stone.setAttribute('stroke-width', '1.2');
        });

        // Set XML namespace and explicit dimensions so that SVG definitions and gradients render properly on canvas
        svgClone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
        const viewBox = svgEl.getAttribute("viewBox") || "0 0 500 500";
        const [, , w, h] = viewBox.split(" ");
        svgClone.setAttribute("width", w || "500");
        svgClone.setAttribute("height", h || "500");

        // Serialize SVG
        const serializer = new XMLSerializer();
        const svgString = serializer.serializeToString(svgClone);
        
        const svgBlob = new Blob([svgString], { type: 'image/svg+xml;charset=utf-8' });
        const URL = window.URL || window.webkitURL || window;
        const blobURL = URL.createObjectURL(svgBlob);

        const image = new Image();
        image.onload = () => {
          // Create canvas to draw the board image in X OGP standards (1.91:1)
          const canvas = document.createElement('canvas');
          const ogpWidth = 1200;
          const ogpHeight = 630;
          canvas.width = ogpWidth;
          canvas.height = ogpHeight;

          const ctx = canvas.getContext('2d');
          if (!ctx) {
            URL.revokeObjectURL(blobURL);
            resolve();
            return;
          }

          // 1. Fill background with Washi Sage Gray (#dcdfdc)
          ctx.fillStyle = '#dcdfdc';
          ctx.fillRect(0, 0, ogpWidth, ogpHeight);

          // --- サイトのデザイン意匠のあしらい（碁盤の邪魔をしない薄い描写） ---
          ctx.lineWidth = 1.5;
          ctx.strokeStyle = 'rgba(37, 53, 48, 0.08)'; // --wc-border (#253530) に透明度をかけたもの

          // 左側のあしらい：石のプレースメントや波紋をイメージした同心円
          ctx.beginPath();
          ctx.arc(100, 315, 180, 0, Math.PI * 2);
          ctx.stroke();

          ctx.beginPath();
          ctx.arc(100, 315, 260, 0, Math.PI * 2);
          ctx.stroke();

          ctx.beginPath();
          ctx.arc(100, 315, 100, 0, Math.PI * 2);
          ctx.stroke();

          // 右側のあしらい：同心円
          ctx.beginPath();
          ctx.arc(1100, 315, 160, 0, Math.PI * 2);
          ctx.stroke();

          ctx.beginPath();
          ctx.arc(1100, 315, 220, 0, Math.PI * 2);
          ctx.stroke();

          ctx.beginPath();
          ctx.arc(1100, 315, 80, 0, Math.PI * 2);
          ctx.stroke();

          // サイトのブランドネーム「K I F U   S T O R E」（Cormorant Garamond 縦書き風）
          ctx.fillStyle = 'rgba(37, 53, 48, 0.35)'; // 邪魔にならないが品良く読める薄い色
          ctx.font = "italic 600 22px 'Cormorant Garamond', 'Shippori Mincho B1', serif";
          ctx.textAlign = "center";
          ctx.textBaseline = "middle";
          
          ctx.save();
          ctx.translate(110, 315);
          ctx.rotate(-Math.PI / 2);
          ctx.fillText("K I F U   S T O R E", 0, 0);
          ctx.restore();

          // 右側に対局情報（対局者、手番、結果など）を表示
          ctx.fillStyle = 'rgba(37, 53, 48, 0.5)';
          ctx.font = "500 16px 'Shippori Mincho B1', 'Noto Sans JP', serif";
          
          let infoText = "";
          if (kifu.black_player && kifu.white_player) {
            const bRank = kifu.black_rank ? ` (${kifu.black_rank})` : "";
            const wRank = kifu.white_rank ? ` (${kifu.white_rank})` : "";
            infoText = `先番: ${kifu.black_player}${bRank}  vs  白番: ${kifu.white_player}${wRank}`;
          } else {
            infoText = kifu.title || "対局棋譜";
          }

          // 日付と結果があれば追加
          let subText = "";
          if (kifu.game_date) {
            const dateStr = kifu.game_date.substring(0, 10);
            subText += `${dateStr}`;
          }
          if (kifu.result) {
            if (subText) subText += "  |  ";
            subText += `結果: ${kifu.result}`;
          }

          // 右側余白に対局情報と日付・結果を上品に縦書き（回転）して配置
          ctx.save();
          ctx.translate(1090, 315);
          ctx.rotate(Math.PI / 2);
          ctx.textAlign = "center";
          ctx.textBaseline = "middle";
          ctx.font = "500 15px 'Shippori Mincho B1', 'Noto Sans JP', serif";
          ctx.fillText(infoText, 0, -12); // 少し左にずらす
          
          if (subText) {
            ctx.fillStyle = 'rgba(37, 53, 48, 0.35)';
            ctx.font = "400 12px 'JetBrains Mono', 'Noto Sans JP', sans-serif";
            ctx.fillText(subText, 0, 12); // 少し右にずらす
          }
          ctx.restore();
          // -------------------------------------------------------------

          // 2. Draw aesthetic Editorial solid shadow card in center (Enlarged with small margins)
          const boardSize = 600; // Expanded to minimize outer padding
          const boardX = (ogpWidth - boardSize) / 2;
          const boardY = (ogpHeight - boardSize) / 2;

          // Solid shadow - matches Vogue/Washi Clay theme shadows
          ctx.fillStyle = '#253530'; // --wc-shadow-dark
          ctx.fillRect(boardX + 10, boardY + 10, boardSize, boardSize); // Compact shadow offset

          // Solid white board outer card
          ctx.fillStyle = '#f4f6f5'; // --wc-surface
          ctx.fillRect(boardX, boardY, boardSize, boardSize);
          
          ctx.strokeStyle = '#253530'; // --wc-border
          ctx.lineWidth = 3.5; // Thicker border for better visibility
          ctx.strokeRect(boardX, boardY, boardSize, boardSize);

          // 3. Draw Go Board SVG onto the card with very small margin (6px instead of 12px)
          ctx.drawImage(image, boardX + 6, boardY + 6, boardSize - 12, boardSize - 12);

          // Convert to blob and upload to backend
          canvas.toBlob(async (blob) => {
            if (!blob) {
              URL.revokeObjectURL(blobURL);
              resolve();
              return;
            }
            try {
              const res = await fetch(`/api/kifus/${kifu.id}/ogp`, {
                method: 'PUT',
                headers: {
                  'Authorization': auth.getHeaders()['Authorization'] || '',
                  'Content-Type': 'image/png'
                },
                body: blob
              });
              if (res.ok) {
                console.log("Successfully uploaded OGP image");
                resolve();
              } else {
                const data = await res.json();
                console.error("Failed to upload OGP image:", data.error);
                reject(new Error(data.error || "OGP画像のアップロードに失敗しました。"));
              }
            } catch (e: any) {
              console.error("Error uploading OGP image:", e);
              reject(e);
            } finally {
              URL.revokeObjectURL(blobURL);
            }
          }, 'image/png');
        };

        image.onloadstart = () => {};
        image.onerror = (err) => {
          console.error("Error loading SVG image", err);
          URL.revokeObjectURL(blobURL);
          reject(new Error("SVG画像のレンダリングに失敗しました。"));
        };

        image.src = blobURL;
      } catch (err: any) {
        console.error("Failed to generate OGP image:", err);
        reject(err);
      }
    });
  }

  async function handleRegenerate() {
    if (loading) return;
    loading = true;
    
    try {
      // 1. Request new share token (URL regeneration)
      const res = await fetch(`/api/kifus/${kifu.id}/share`, {
        method: 'POST',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          disable: false, // Ensure active sharing
          expires_in_days: null // Never expires by default
        })
      });

      const updated = await res.json();
      if (!res.ok) {
        throw new Error(updated.error || "URLの再発行に失敗しました。");
      }

      // 2. Generate and upload new OGP image immediately
      await generateAndUploadOgp();

      // 3. Update parent state
      onUpdate(updated);

      const M = getM();
      if (M) {
        M.toast({ html: 'URLとOGP画像を再発行しました！', classes: 'green darken-1' });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red darken-1' });
      }
    } finally {
      loading = false;
    }
  }

  // Generate QR code when shareUrl changes
  $effect(() => {
    if (shareUrl) {
      // Use clean settings, high contrast for reliable scanning
      QRCode.toString(shareUrl, { type: 'svg', width: 180, margin: 1 }, (err, string) => {
        if (err) {
          console.error(err);
          qrCodeSvg = "";
        } else {
          qrCodeSvg = string;
        }
      });
    } else {
      qrCodeSvg = "";
    }
  });

  async function handleTogglePrivacy(setPublic: boolean) {
    if (loading) return;
    loading = true;
    const nextIsPrivate = !setPublic;
    try {
      const res = await fetch(`/api/kifus/${kifu.id}/privacy`, {
        method: 'PUT',
        headers: auth.getHeaders(),
        body: JSON.stringify({
          is_private: nextIsPrivate
        })
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "公開設定の更新に失敗しました。");
      }

      isPrivate = nextIsPrivate;
      onUpdate({ ...kifu, is_private: nextIsPrivate });

      // Generate and upload new OGP image immediately on privacy toggle
      await generateAndUploadOgp();

      const M = getM();
      if (M) {
        M.toast({ 
          html: nextIsPrivate ? '限定公開に変更しました' : '一般公開に変更し、OGP画像を更新しました！', 
          classes: 'green darken-1' 
        });
      }
    } catch (err: any) {
      const M = getM();
      if (M) {
        M.toast({ html: 'エラー: ' + err.message, classes: 'red darken-1' });
      }
    } finally {
      loading = false;
    }
  }

  async function handleCopy() {
    if (!shareUrl) return;
    try {
      await navigator.clipboard.writeText(shareUrl);
      copySuccess = true;
      setTimeout(() => { copySuccess = false; }, 2000);
      const M = getM();
      if (M) {
        M.toast({ html: 'リンクをコピーしました', classes: 'grey darken-3' });
      }
    } catch (err) {
      console.error("Failed to copy", err);
    }
  }
</script>

<!-- Backdrop click triggers onClose -->
<div class="share-modal-backdrop animate-fade-in" onclick={onClose} aria-hidden="true">
  <!-- Content click propagation stopped to avoid closing -->
  <div class="share-modal-content nm-modal" onclick={(e) => e.stopPropagation()} aria-hidden="true">
    
    <!-- Header with Washi Clay Design Style -->
    <div class="share-modal-header">
      <span class="share-modal-title font-mincho">
        <i class="material-icons share-modal-title-icon">share</i>
        棋譜を共有する
      </span>
      <p class="share-modal-subtitle">
        添削の依頼や閲覧用のリンクとQRコードです。
      </p>
    </div>

    <div class="share-active-box center-align">
      <!-- QR Container: High contrast white background for perfect scans, with solid sharp box-shadow -->
      <div class="qr-container-wrapper">
        <div class="qr-container">
          {#if qrCodeSvg}
            <!-- eslint-disable-next-line svelte/no-at-html-tags -->
            {@html qrCodeSvg}
          {:else}
            <div class="qr-placeholder">
              <div class="nm-spinner"></div>
              <p>QRコード生成中...</p>
            </div>
          {/if}
        </div>
      </div>

      <!-- Copy Link Container: One-piece Ticket style URL box -->
      <div class="url-ticket-box">
        <div class="url-ticket-text font-mono">{shareUrl}</div>
        <button class="url-ticket-btn font-sans" onclick={handleCopy} disabled={copySuccess}>
          {#if copySuccess}
            <i class="material-icons" style="font-size: 0.9rem; margin-right: 4px; vertical-align: middle;">check</i>COPIED
          {:else}
            COPY
          {/if}
        </button>
      </div>

      <!-- Regenerate Action Button: Matches Vogue solid button, fits below URL box -->
      <div class="regenerate-action-container">
        <button class="regenerate-btn font-sans" onclick={handleRegenerate} disabled={loading}>
          <i class="material-icons btn-icon" class:spin={loading}>{loading ? 'sync' : 'refresh'}</i>
          URL再発行 & OGP画像再生成
        </button>
      </div>

      <!-- Privacy Selector: Segmented editorial block -->
      <div class="privacy-section">
        <div class="visibility-selector">
          <button 
            type="button" 
            class="visibility-tab font-sans" 
            class:active={isPrivate} 
            onclick={() => handleTogglePrivacy(false)} 
            disabled={loading}
          >
            <i class="material-icons tab-icon">lock_outline</i>
            限定公開
          </button>
          <button 
            type="button" 
            class="visibility-tab font-sans" 
            class:active={!isPrivate} 
            onclick={() => handleTogglePrivacy(true)} 
            disabled={loading}
          >
            <i class="material-icons tab-icon">public</i>
            一般公開
          </button>
        </div>
        
        <div class="privacy-explanation-box">
          <p class="privacy-desc">
            {#if isPrivate}
              <strong>限定公開（リンクを知っている人のみ閲覧可能）</strong><br>
              あなたのプロフィールや一般公開のリスト、検索エンジンには掲載されません。
            {:else}
              <strong>一般公開（全員に公開）</strong><br>
              誰でも閲覧可能で、あなたの公開棋譜一覧や検索エンジンにも登録されます。
            {/if}
          </p>
        </div>
      </div>
    </div>

    <!-- Actions Footer -->
    <div class="share-modal-footer">
      <button class="close-btn" onclick={onClose}>閉じる</button>
    </div>
  </div>
</div>

<style>
  .share-modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background: rgba(37, 53, 48, 0.4); /* Sage Green translucent backdrop */
    backdrop-filter: var(--wc-glass-blur);
    -webkit-backdrop-filter: var(--wc-glass-blur);
    z-index: 1000;
    display: flex;
    justify-content: center;
    align-items: center;
    padding: 16px;
  }

  .share-modal-content {
    box-sizing: border-box;
    width: 100%;
    max-width: 480px;
    background: var(--wc-surface) !important;
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    box-shadow: 8px 8px 0px var(--wc-shadow-dark) !important;
    padding: 32px 28px !important;
    max-height: calc(100vh - 40px);
    overflow-y: auto;
  }

  .share-modal-header {
    margin-bottom: 24px;
    text-align: left;
  }

  .share-modal-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 800;
    color: var(--wc-text);
    font-size: 1.3rem;
    margin-bottom: 8px;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .share-modal-title-icon {
    font-size: 1.5rem;
    color: var(--wc-accent);
  }

  .share-modal-subtitle {
    margin: 0;
    font-size: 0.82rem;
    color: var(--wc-text-muted);
    line-height: 1.6;
    font-family: 'DM Sans', sans-serif;
  }

  /* QR Code Wrapper - Vogue sharp frame */
  .qr-container-wrapper {
    display: flex;
    justify-content: center;
    margin-bottom: 24px;
  }

  .qr-container {
    display: inline-flex;
    justify-content: center;
    align-items: center;
    padding: 12px;
    background: #ffffff; /* Must remain white for scan */
    border-radius: 0px !important; 
    border: 1.5px solid var(--wc-border) !important;
    box-shadow: 4px 4px 0px var(--wc-shadow-dark) !important;
    transition: var(--wc-transition-fast);
  }

  .qr-container:hover {
    transform: translate(-2px, -2px);
    box-shadow: 6px 6px 0px var(--wc-shadow-dark) !important;
  }

  /* Make QR Code SVG scale smoothly */
  .qr-container :global(svg) {
    display: block;
    width: 180px;
    height: 180px;
    max-width: 100%;
    max-height: 100%;
    transition: var(--wc-transition-fast);
  }

  .qr-placeholder {
    width: 180px;
    height: 180px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    color: var(--wc-text-muted);
    font-size: 0.85rem;
  }

  /* Ticket Box style URL slot */
  .url-ticket-box {
    display: flex;
    align-items: stretch;
    border: 1.5px solid var(--wc-text);
    background: rgba(245, 240, 232, 0.4);
    margin-bottom: 24px;
    position: relative;
    box-shadow: 3px 3px 0px var(--wc-shadow-dark);
  }

  .url-ticket-text {
    flex-grow: 1;
    padding: 11px 14px;
    font-size: 0.82rem;
    color: var(--wc-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
    user-select: all;
  }

  .url-ticket-btn {
    border: none;
    border-left: 1.5px solid var(--wc-text);
    background: var(--wc-surface-alt);
    color: var(--wc-text);
    padding: 0 18px;
    font-weight: 700;
    font-size: 0.78rem;
    letter-spacing: 0.05em;
    cursor: pointer;
    transition: var(--wc-transition-fast);
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .url-ticket-btn:hover:not(:disabled) {
    background: var(--wc-accent-soft);
    color: var(--wc-accent);
  }

  .url-ticket-btn:active {
    background: rgba(37, 53, 48, 0.1);
  }

  .url-ticket-btn:disabled {
    background: var(--wc-surface);
    color: var(--wc-mid);
    cursor: default;
  }

  .regenerate-action-container {
    margin-bottom: 24px;
    text-align: left;
  }

  .regenerate-btn {
    width: 100%;
    border-radius: 0px !important;
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    box-shadow: 3px 3px 0px var(--wc-text) !important;
    padding: 10px 16px;
    font-weight: 700;
    font-size: 0.82rem;
    letter-spacing: 0.03em;
    cursor: pointer;
    transition: var(--wc-transition-fast);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
  }

  .regenerate-btn:hover:not(:disabled) {
    transform: translate(-1px, -1px);
    box-shadow: 4px 4px 0px var(--wc-text) !important;
    background: var(--wc-surface-alt) !important;
  }

  .regenerate-btn:active:not(:disabled) {
    transform: translate(1px, 1px);
    box-shadow: 1px 1px 0px var(--wc-text) !important;
  }

  .regenerate-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    box-shadow: none !important;
    transform: none !important;
  }

  .btn-icon {
    font-size: 1.1rem;
  }

  .spin {
    animation: rotate 1s linear infinite;
  }

  @keyframes rotate {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  /* Privacy Selector - Vogue Segmented Tabs */
  .privacy-section {
    display: flex;
    flex-direction: column;
    margin-bottom: 8px;
    border: 1.5px solid var(--wc-border);
  }

  .visibility-selector {
    display: flex;
    border-bottom: 1.5px solid var(--wc-border);
  }

  .visibility-tab {
    flex: 1;
    border: none;
    background: var(--wc-surface);
    color: var(--wc-text-muted);
    padding: 12px 16px;
    font-size: 0.85rem;
    font-weight: 700;
    letter-spacing: 0.02em;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    transition: var(--wc-transition-fast);
  }

  .visibility-tab:first-child {
    border-right: 1.5px solid var(--wc-border);
  }

  .visibility-tab.active {
    background: var(--wc-text);
    color: var(--wc-surface) !important;
  }

  .visibility-tab:hover:not(.active):not(:disabled) {
    background: var(--wc-surface-alt);
    color: var(--wc-text);
  }

  .tab-icon {
    font-size: 1.1rem;
  }

  .privacy-explanation-box {
    padding: 16px;
    background: rgba(245, 240, 232, 0.25);
    text-align: left;
  }

  .privacy-desc {
    margin: 0;
    font-size: 0.8rem;
    line-height: 1.6;
    color: var(--wc-text-muted);
    font-family: 'DM Sans', 'Noto Sans JP', sans-serif;
  }

  .privacy-desc strong {
    color: var(--wc-text);
  }

  .share-modal-footer {
    padding: 20px 0 0 0;
    display: flex;
    justify-content: flex-end;
    border-top: 1.5px solid var(--wc-border);
    margin-top: 24px;
  }

  .close-btn {
    border-radius: 0px !important; 
    border: 1.5px solid var(--wc-text) !important;
    background: var(--wc-surface) !important;
    color: var(--wc-text) !important;
    box-shadow: 3px 3px 0px var(--wc-text) !important;
    padding: 8px 24px;
    font-weight: 700;
    font-size: 0.88rem;
    font-family: 'DM Sans', sans-serif;
    cursor: pointer;
    transition: var(--wc-transition-fast);
  }

  .close-btn:hover {
    transform: translate(-1px, -1px);
    box-shadow: 4px 4px 0px var(--wc-text) !important;
    background: var(--wc-surface-alt) !important;
  }

  .close-btn:active {
    transform: translate(1px, 1px);
    box-shadow: 1px 1px 0px var(--wc-text) !important;
  }

  /* Font helper styles */
  .font-mincho {
    font-family: 'Shippori Mincho B1', serif;
  }

  .font-mono {
    font-family: 'JetBrains Mono', monospace;
  }

  /* Animation and responsive styles */
  .animate-fade-in {
    animation: fadeIn 0.25s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @media only screen and (max-width: 480px) {
    .url-ticket-box {
      flex-direction: column;
    }
    .url-ticket-btn {
      border-left: none;
      border-top: 1.5px solid var(--wc-text);
      padding: 10px 0;
    }
  }

  /* 高さまたは幅が狭い場合のスタイル微調整（縦レスポンシブ） */
  @media (max-height: 640px) or (max-width: 480px) {
    .share-modal-content {
      padding: 16px 20px !important;
    }

    .share-modal-header {
      margin-bottom: 12px;
    }

    .share-modal-title {
      font-size: 1.1rem;
      margin-bottom: 4px;
    }

    .share-modal-subtitle {
      font-size: 0.75rem;
    }

    .qr-container-wrapper {
      margin-bottom: 12px;
    }

    .qr-container {
      padding: 8px;
    }

    .qr-placeholder {
      width: 120px;
      height: 120px;
      font-size: 0.75rem;
    }

    .qr-container :global(svg) {
      width: 120px;
      height: 120px;
    }

    .url-ticket-box {
      margin-bottom: 12px;
      box-shadow: 2px 2px 0px var(--wc-shadow-dark);
    }

    .url-ticket-text {
      padding: 8px 12px;
      font-size: 0.75rem;
    }

    .url-ticket-btn {
      padding: 0 12px;
      font-size: 0.72rem;
    }

    .regenerate-action-container {
      margin-bottom: 12px;
    }

    .regenerate-btn {
      padding: 8px 12px;
      font-size: 0.75rem;
      box-shadow: 2px 2px 0px var(--wc-text) !important;
    }

    .privacy-section {
      margin-bottom: 4px;
    }

    .visibility-tab {
      padding: 8px 12px;
      font-size: 0.78rem;
    }

    .privacy-explanation-box {
      padding: 10px 12px;
    }

    .privacy-desc {
      font-size: 0.72rem;
      line-height: 1.4;
    }

    .share-modal-footer {
      margin-top: 12px;
      padding-top: 12px;
    }

    .close-btn {
      padding: 6px 18px;
      font-size: 0.8rem;
      box-shadow: 2px 2px 0px var(--wc-text) !important;
    }
  }
</style>
