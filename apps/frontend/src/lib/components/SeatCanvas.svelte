<script lang="ts">
  import { onMount } from 'svelte';
  import type { Seat } from '$lib/api/types';

  let {
    seats,
    selectedSeatIDs,
    onToggle,
    sectionID = 'A',
    zoomLevel = 1,
    accessibleOnly = false
  }: {
    seats: Seat[];
    selectedSeatIDs: Set<string>;
    onToggle: (seat: Seat) => void;
    sectionID?: string;
    zoomLevel?: number;
    accessibleOnly?: boolean;
  } = $props();

  let container = $state<HTMLDivElement>();
  let canvas = $state<HTMLCanvasElement>();
  let width = $state(700);
  let height = $state(360);
  let devicePixelRatio = $state(1);
  let hoveredSeat = $state<Seat | null>(null);

  const colors: Record<string, string> = {
    AVAILABLE: '#8FA3B8',
    SELECTED: '#9F1D2F',
    HELD: '#FF5C5C',
    RESERVED: '#2E7D5B',
    UNKNOWN: '#273244'
  };

  let minX = $state(0),
    maxX = $state(0),
    minY = $state(0),
    maxY = $state(0);
  let scale = $state(1),
    offsetX = $state(0),
    offsetY = $state(0);
  const SEAT_SIZE = 28;

  // Use canvas only for dense sections; SVG keeps low-density seats accessible.
  const CANVAS_SEAT_THRESHOLD = 1000;
  let useCanvas = $derived(seats.length > CANVAS_SEAT_THRESHOLD);
  let seatRegions = $derived(
    seats.map((seat) => ({
      seat,
      x: offsetX + (seat.x - minX) * scale,
      y: offsetY + (seat.y - minY) * scale,
      size: SEAT_SIZE * scale
    }))
  );
  let frameHandle: number | undefined;

  function seatPosition(seat: Seat) {
    return {
      x: offsetX + (seat.x - minX) * scale,
      y: offsetY + (seat.y - minY) * scale,
      size: SEAT_SIZE * scale
    };
  }

  function seatFill(seat: Seat, selected: boolean, isDimmed: boolean) {
    if (isDimmed) return 'rgba(255,255,255,0.05)';
    return selected ? colors.SELECTED : colors[seat.status];
  }

  function seatStroke(
    seat: Seat,
    selected: boolean,
    isHovered: boolean,
    isDimmed: boolean
  ) {
    if (isDimmed) return 'rgba(255,255,255,0.02)';
    if (selected || isHovered) return '#FFFFFF';
    if (seat.status === 'UNKNOWN') return '#4B5563';
    return 'rgba(255,255,255,0.1)';
  }

  function seatCursor(seat: Seat) {
    return seat.status !== 'RESERVED' && (!accessibleOnly || seat.accessibility)
      ? 'pointer'
      : 'default';
  }

  function handleSeatClick(seat: Seat) {
    if (seat.status === 'RESERVED') return;
    if (accessibleOnly && !seat.accessibility) return;
    onToggle(seat);
  }

  $effect(() => {
    if (seats.length > 0) {
      let nextMinX = seats[0].x;
      let nextMaxX = seats[0].x;
      let nextMinY = seats[0].y;
      let nextMaxY = seats[0].y;
      for (const seat of seats) {
        if (seat.x < nextMinX) nextMinX = seat.x;
        if (seat.x > nextMaxX) nextMaxX = seat.x;
        if (seat.y < nextMinY) nextMinY = seat.y;
        if (seat.y > nextMaxY) nextMaxY = seat.y;
      }
      minX = nextMinX;
      maxX = nextMaxX;
      minY = nextMinY;
      maxY = nextMaxY;

      const gridWidth = maxX - minX + SEAT_SIZE;
      const gridHeight = maxY - minY + SEAT_SIZE;

      const scaleX = (width - 100) / gridWidth;
      const scaleY = (height - 140) / gridHeight;
      scale = Math.min(scaleX, scaleY, 1.2) * zoomLevel;

      offsetX = (width - gridWidth * scale) / 2;
      offsetY = (height - gridHeight * scale) / 2;
    }
  });

  function roundRect(
    ctx: CanvasRenderingContext2D,
    x: number,
    y: number,
    w: number,
    h: number,
    r: number
  ) {
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + w, y, x + w, y + h, r);
    ctx.arcTo(x + w, y + h, x, y + h, r);
    ctx.arcTo(x, y + h, x, y, r);
    ctx.arcTo(x, y, x + w, y, r);
    ctx.closePath();
  }

  function draw() {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    ctx.save();
    ctx.scale(devicePixelRatio, devicePixelRatio);

    for (const region of seatRegions) {
      const { seat, x: sx, y: sy, size } = region;
      const selected = selectedSeatIDs.has(seat.seat_id);
      const isHovered = hoveredSeat?.seat_id === seat.seat_id;

      const radius = 6 * scale;

      if (
        isHovered &&
        seat.status !== 'RESERVED' &&
        (!accessibleOnly || seat.accessibility)
      ) {
        ctx.shadowColor = 'rgba(255, 255, 255, 0.6)';
        ctx.shadowBlur = 10 * scale;
      } else if (selected || seat.status === 'HELD') {
        ctx.shadowColor = selected
          ? 'rgba(159, 29, 47, 0.62)'
          : 'rgba(255, 92, 92, 0.62)';
        ctx.shadowBlur = 9 * scale;
      } else {
        ctx.shadowBlur = 0;
      }

      const isDimmed = accessibleOnly && !seat.accessibility;
      ctx.fillStyle = isDimmed
        ? 'rgba(255,255,255,0.05)'
        : selected
          ? colors.SELECTED
          : colors[seat.status];
      roundRect(ctx, sx, sy, size, size, radius);
      ctx.fill();

      ctx.shadowBlur = 0;
      ctx.lineWidth = selected || (isHovered && !isDimmed) ? 2 : 1;
      ctx.strokeStyle = isDimmed
        ? 'rgba(255,255,255,0.02)'
        : selected || isHovered
          ? '#FFFFFF'
          : seat.status === 'UNKNOWN'
            ? '#4B5563'
            : 'rgba(255,255,255,0.1)';
      ctx.stroke();
    }
    ctx.restore();
  }

  function requestDraw() {
    if (!useCanvas || typeof requestAnimationFrame === 'undefined') return;
    if (frameHandle !== undefined) return;
    frameHandle = requestAnimationFrame(() => {
      frameHandle = undefined;
      draw();
    });
  }

  function getHitSeat(event: MouseEvent) {
    if (!canvas || seats.length === 0) return null;
    const rect = canvas.getBoundingClientRect();
    const clickX = ((event.clientX - rect.left) / rect.width) * width;
    const clickY = ((event.clientY - rect.top) / rect.height) * height;

    for (let i = seatRegions.length - 1; i >= 0; i -= 1) {
      const region = seatRegions[i];
      if (
        clickX >= region.x &&
        clickX <= region.x + region.size &&
        clickY >= region.y &&
        clickY <= region.y + region.size
      ) {
        return region.seat;
      }
    }
    return null;
  }

  function handleClick(event: MouseEvent) {
    const hit = getHitSeat(event);
    if (hit && hit.status !== 'RESERVED') {
      if (accessibleOnly && !hit.accessibility) return;
      onToggle(hit);
    }
  }

  function handleMouseMove(event: MouseEvent) {
    const hit = getHitSeat(event);
    if (hoveredSeat?.seat_id !== hit?.seat_id) {
      hoveredSeat = hit;
      requestDraw();
    }
  }

  function handleMouseLeave() {
    if (hoveredSeat) {
      hoveredSeat = null;
      requestDraw();
    }
  }

  onMount(() => {
    devicePixelRatio = window.devicePixelRatio || 1;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0) {
          width = entry.contentRect.width;
          height = entry.contentRect.height;
          requestDraw();
        }
      }
    });
    if (container) observer.observe(container);

    return () => {
      observer.disconnect();
      if (frameHandle !== undefined) {
        cancelAnimationFrame(frameHandle);
      }
    };
  });

  $effect(() => {
    selectedSeatIDs.size;
    scale;
    offsetX;
    offsetY;
    hoveredSeat;
    zoomLevel;
    accessibleOnly;
    if (seats && canvas) {
      requestDraw();
    }
  });
</script>

<div
  bind:this={container}
  class="border-line bg-panel relative h-full min-h-[440px] overflow-hidden rounded-sm border shadow-lg"
>
  {#if useCanvas}
    <canvas
      bind:this={canvas}
      class="absolute inset-0 z-10 transition-opacity duration-300 hover:opacity-95"
      style="width: {width}px; height: {height}px; display: block; cursor: {hoveredSeat &&
      hoveredSeat.status !== 'RESERVED' &&
      (!accessibleOnly || hoveredSeat.accessibility)
        ? 'pointer'
        : 'default'};"
      width={width * devicePixelRatio}
      height={height * devicePixelRatio}
      onclick={handleClick}
      onmousemove={handleMouseMove}
      onmouseleave={handleMouseLeave}
      aria-label="Interactive seat map"
    ></canvas>
  {:else}
    <svg
      class="absolute inset-0 z-10 transition-opacity duration-300 hover:opacity-95"
      {width}
      {height}
      viewBox="0 0 {width} {height}"
      role="group"
      aria-label="Interactive seat map"
    >
      {#each seats as seat (seat.seat_id)}
        {@const selected = selectedSeatIDs.has(seat.seat_id)}
        {@const isDimmed = accessibleOnly && !seat.accessibility}
        {@const isHovered = hoveredSeat?.seat_id === seat.seat_id}
        {@const pos = seatPosition(seat)}
        <rect
          x={pos.x}
          y={pos.y}
          width={pos.size}
          height={pos.size}
          rx={6 * scale}
          fill={seatFill(seat, selected, isDimmed)}
          stroke={seatStroke(seat, selected, isHovered, isDimmed)}
          stroke-width={selected || (isHovered && !isDimmed) ? 2 : 1}
          style="cursor: {seatCursor(seat)}; transition: fill 0.15s ease;"
          role="button"
          aria-label={`Seat ${seat.seat_id}, ${selected ? 'selected' : seat.status.toLowerCase()}`}
          tabindex="0"
          onclick={() => handleSeatClick(seat)}
          onkeydown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              handleSeatClick(seat);
            }
          }}
          onmouseenter={() => (hoveredSeat = seat)}
          onmouseleave={() => (hoveredSeat = null)}
        />
      {/each}
    </svg>
  {/if}

  <div
    class="pointer-events-none absolute inset-0 z-0 flex flex-col items-center justify-between py-6"
  >
    <div class="flex flex-col items-center opacity-60">
      <div class="bg-ink/20 mb-2 h-1 w-32 rounded-full"></div>
      <span class="text-inkMuted text-xs font-black tracking-[0.3em] uppercase"
        >Section {sectionID}</span
      >
    </div>

    <div class="relative bottom-[-20px] flex flex-col items-center">
      <span
        class="text-signal mb-2 text-xs font-black tracking-[0.6em] uppercase"
        >Stage</span
      >
      <div
        class="border-signal/50 h-16 w-80 rounded-t-[100%] border-t-[3px] bg-[linear-gradient(0deg,rgba(242,184,75,0.18),transparent)]"
      ></div>
    </div>
  </div>
</div>
