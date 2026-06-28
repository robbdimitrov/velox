<script lang="ts">
  import { onMount } from 'svelte';
  import type { Seat } from '$lib/api/types';

  let {
    seats,
    selectedSeatIDs,
    onToggle
  }: {
    seats: Seat[];
    selectedSeatIDs: Set<string>;
    onToggle: (seat: Seat) => void;
  } = $props();

  let container = $state<HTMLDivElement>();
  let canvas = $state<HTMLCanvasElement>();
  let width = $state(700);
  let height = $state(360);
  let devicePixelRatio = $state(1);
  let hoveredSeat = $state<Seat | null>(null);

  const colors: Record<string, string> = {
    AVAILABLE: '#94A3B8',
    SELECTED: '#FACC15',
    HELD: '#EF4444',
    SOLD: '#0B0C10',
    UNKNOWN: '#1F2937'
  };

  let minX = $state(0), maxX = $state(0), minY = $state(0), maxY = $state(0);
  let scale = $state(1), offsetX = $state(0), offsetY = $state(0);
  const SEAT_SIZE = 28;

  $effect(() => {
    if (seats.length > 0) {
      minX = Math.min(...seats.map(s => s.x));
      maxX = Math.max(...seats.map(s => s.x));
      minY = Math.min(...seats.map(s => s.y));
      maxY = Math.max(...seats.map(s => s.y));
      
      const gridWidth = maxX - minX + SEAT_SIZE;
      const gridHeight = maxY - minY + SEAT_SIZE;
      
      const scaleX = (width - 100) / gridWidth;
      const scaleY = (height - 140) / gridHeight;
      scale = Math.min(scaleX, scaleY, 1.2); 
      
      offsetX = (width - (gridWidth * scale)) / 2;
      offsetY = (height - (gridHeight * scale)) / 2;
    }
  });

  function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
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

    for (const seat of seats) {
      const selected = selectedSeatIDs.has(seat.seat_id);
      const isHovered = hoveredSeat?.seat_id === seat.seat_id;
      
      const sx = offsetX + (seat.x - minX) * scale;
      const sy = offsetY + (seat.y - minY) * scale;
      const size = SEAT_SIZE * scale;
      const radius = 6 * scale;

      if (isHovered && seat.status !== 'SOLD') {
        ctx.shadowColor = 'rgba(255, 255, 255, 0.6)';
        ctx.shadowBlur = 15 * scale;
      } else if (selected || seat.status === 'HELD') {
        ctx.shadowColor = selected ? 'rgba(250, 204, 21, 0.8)' : 'rgba(239, 68, 68, 0.8)';
        ctx.shadowBlur = 12 * scale;
      } else {
        ctx.shadowBlur = 0;
      }

      ctx.fillStyle = selected ? colors.SELECTED : colors[seat.status];
      roundRect(ctx, sx, sy, size, size, radius);
      ctx.fill();
      
      ctx.shadowBlur = 0;
      ctx.lineWidth = selected || isHovered ? 2 : 1;
      ctx.strokeStyle = selected || isHovered
        ? '#FFFFFF'
        : seat.status === 'UNKNOWN'
          ? '#4B5563'
          : 'rgba(255,255,255,0.1)';
      ctx.stroke();
    }
    ctx.restore();
  }

  function getHitSeat(event: MouseEvent) {
    if (!canvas || seats.length === 0) return null;
    const rect = canvas.getBoundingClientRect();
    const clickX = (event.clientX - rect.left) / rect.width * width;
    const clickY = (event.clientY - rect.top) / rect.height * height;
    
    return seats.find((seat) => {
      const sx = offsetX + (seat.x - minX) * scale;
      const sy = offsetY + (seat.y - minY) * scale;
      const size = SEAT_SIZE * scale;
      return clickX >= sx && clickX <= sx + size && clickY >= sy && clickY <= sy + size;
    }) || null;
  }

  function handleClick(event: MouseEvent) {
    const hit = getHitSeat(event);
    if (hit && hit.status !== 'SOLD') onToggle(hit);
  }

  function handleMouseMove(event: MouseEvent) {
    const hit = getHitSeat(event);
    if (hoveredSeat?.seat_id !== hit?.seat_id) {
      hoveredSeat = hit;
      draw();
    }
  }

  function handleMouseLeave() {
    if (hoveredSeat) {
      hoveredSeat = null;
      draw();
    }
  }

  onMount(() => {
    devicePixelRatio = window.devicePixelRatio || 1;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0) {
          width = entry.contentRect.width;
          height = entry.contentRect.height;
          draw();
        }
      }
    });
    if (container) observer.observe(container);
    
    return () => observer.disconnect();
  });

  $effect(() => {
    selectedSeatIDs.size;
    scale; offsetX; offsetY; hoveredSeat;
    if (seats && canvas) {
      draw();
    }
  });
</script>

<div bind:this={container} class="relative min-h-[420px] h-full rounded-2xl border border-white/10 bg-black/40 shadow-lg overflow-hidden backdrop-blur-md">
  <canvas
    bind:this={canvas}
    class="absolute inset-0 z-10 transition-opacity duration-300 hover:opacity-95"
    style="width: {width}px; height: {height}px; display: block; cursor: {hoveredSeat && hoveredSeat.status !== 'SOLD' ? 'pointer' : 'default'};"
    width={width * devicePixelRatio}
    height={height * devicePixelRatio}
    onclick={handleClick}
    onmousemove={handleMouseMove}
    onmouseleave={handleMouseLeave}
    aria-label="Interactive seat map"
  ></canvas>
  
  <div class="absolute inset-0 pointer-events-none flex flex-col justify-between items-center py-6 z-0">
    <div class="flex flex-col items-center opacity-60">
      <div class="h-1 w-32 bg-white/20 rounded-full mb-2"></div>
      <span class="text-xs font-black uppercase tracking-[0.3em] text-white/60">Section A</span>
    </div>
    
    <div class="flex flex-col items-center relative bottom-[-20px]">
      <span class="text-xs font-black uppercase tracking-[0.6em] text-signal mb-2 drop-shadow-[0_0_8px_rgba(250,204,21,0.8)]">Stage</span>
      <div class="w-80 h-16 border-t-[3px] border-signal/40 rounded-t-[100%] bg-gradient-to-t from-signal/20 to-transparent shadow-[0_-15px_30px_rgba(250,204,21,0.15)]"></div>
    </div>
  </div>
</div>
