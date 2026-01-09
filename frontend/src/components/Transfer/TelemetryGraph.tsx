/**
 * TelemetryGraph Component
 * Term-Phase 2: Real-time SVG Sparkline
 * 
 * A high-performance "Heartbeat" graph showing transfer speed over time.
 * Custom SVG path generation - no heavy chart libraries.
 */

import { memo, useMemo } from 'react';

interface TelemetryGraphProps {
  data: number[];
  width?: number;
  height?: number;
  className?: string;
}

/**
 * Generate SVG path from data points
 * Uses smooth bezier curves for fluid appearance
 */
function generatePath(
  data: number[],
  width: number,
  height: number,
  padding: number = 4
): string {
  if (data.length < 2) return '';
  
  const maxValue = Math.max(...data, 1);
  const xStep = width / (data.length - 1);
  const yScale = (height - padding * 2) / maxValue;
  
  // Generate points
  const points = data.map((value, i) => ({
    x: i * xStep,
    y: height - padding - (value * yScale),
  }));
  
  // Build smooth path with quadratic curves
  let path = `M ${points[0].x},${points[0].y}`;
  
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const cpx = (prev.x + curr.x) / 2;
    path += ` Q ${cpx},${prev.y} ${cpx},${(prev.y + curr.y) / 2}`;
    path += ` Q ${cpx},${curr.y} ${curr.x},${curr.y}`;
  }
  
  return path;
}

/**
 * Generate fill path (closed polygon for gradient fill)
 */
function generateFillPath(
  data: number[],
  width: number,
  height: number,
  padding: number = 4
): string {
  const linePath = generatePath(data, width, height, padding);
  if (!linePath) return '';
  
  // Close the path at the bottom
  return `${linePath} L ${width},${height} L 0,${height} Z`;
}

/**
 * TelemetryGraph - Memoized for performance
 * Renders a real-time sparkline with gradient fill
 */
export const TelemetryGraph = memo(function TelemetryGraph({
  data,
  width = 200,
  height = 48,
  className = '',
}: TelemetryGraphProps) {
  // Generate paths (memoized)
  const { linePath, fillPath } = useMemo(() => ({
    linePath: generatePath(data, width, height),
    fillPath: generateFillPath(data, width, height),
  }), [data, width, height]);

  const gradientId = useMemo(() => `telemetry-gradient-${Math.random().toString(36).slice(2)}`, []);

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className={`transform-gpu ${className}`}
      style={{ willChange: 'contents' }}
    >
      <defs>
        {/* Vertical gradient for fill */}
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#00E5FF" stopOpacity="0.4" />
          <stop offset="100%" stopColor="#00E5FF" stopOpacity="0" />
        </linearGradient>
        
        {/* Glow filter */}
        <filter id="glow" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="2" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>
      
      {/* Grid lines */}
      <g opacity="0.1">
        <line x1="0" y1={height * 0.25} x2={width} y2={height * 0.25} stroke="#fff" strokeWidth="1" />
        <line x1="0" y1={height * 0.5} x2={width} y2={height * 0.5} stroke="#fff" strokeWidth="1" />
        <line x1="0" y1={height * 0.75} x2={width} y2={height * 0.75} stroke="#fff" strokeWidth="1" />
      </g>
      
      {/* Fill area */}
      {fillPath && (
        <path
          d={fillPath}
          fill={`url(#${gradientId})`}
          className="transition-all duration-100"
        />
      )}
      
      {/* Main line */}
      {linePath && (
        <path
          d={linePath}
          fill="none"
          stroke="#00E5FF"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          filter="url(#glow)"
          className="transition-all duration-100"
        />
      )}
      
      {/* Leading dot (current value) */}
      {data.length > 0 && (
        <circle
          cx={width}
          cy={height - 4 - (data[data.length - 1] / Math.max(...data, 1)) * (height - 8)}
          r="3"
          fill="#fff"
          className="animate-pulse"
        />
      )}
    </svg>
  );
});
