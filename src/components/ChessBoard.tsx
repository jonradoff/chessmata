import { useMemo, useRef, useState } from 'react'
import { useFrame } from '@react-three/fiber'
import type { ThreeEvent } from '@react-three/fiber'
import * as THREE from 'three'
import type { useGameState } from '../hooks/useGameState'
import type { BoardMaterial } from '../hooks/useSettings'
import type { OnlineContext } from './ChessScene'
import { isValidMove, toAlgebraic } from '../utils/chessRules'
import { parseFENState } from '../utils/fenParser'

interface ChessBoardProps {
  morphProgress: React.MutableRefObject<{ value: number }>
  gameState: ReturnType<typeof useGameState>
  boardMaterial: BoardMaterial
  onlineContext?: OnlineContext
}

// Build board from FEN state for move validation
function boardFromFEN(fen: string) {
  const state = parseFENState(fen)
  return {
    pieces: state.pieces.map(p => ({
      type: p.type,
      isWhite: p.isWhite,
      file: p.file,
      rank: p.rank,
    })),
    whiteToMove: state.whiteToMove,
    castlingRights: state.castlingRights,
    enPassantSquare: state.enPassantSquare,
  }
}

export function ChessBoard({ morphProgress, gameState, boardMaterial, onlineContext }: ChessBoardProps) {
  const squaresRef = useRef<THREE.Group>(null)
  const boardSize = 8
  const squareSize = 1

  const squares = useMemo(() => {
    const result: { position: [number, number, number]; isLight: boolean; file: number; rank: number }[] = []

    for (let rank = 0; rank < boardSize; rank++) {
      for (let file = 0; file < boardSize; file++) {
        const x = (file - boardSize / 2 + 0.5) * squareSize
        const z = (rank - boardSize / 2 + 0.5) * squareSize
        const isLight = (rank + file) % 2 === 0
        result.push({ position: [x, 0, z], isLight, file, rank })
      }
    }
    return result
  }, [])

  const handleSquareClick = async (file: number, rank: number) => {
    const { selectedPieceId, movePiece, getPieceAt, selectPiece, pieces } = gameState

    // Check if we're in an online game
    const isOnlineGame = onlineContext?.sessionId != null
    const gameStatus = onlineContext?.gameStatus

    // Don't allow any moves if no game session exists
    if (!isOnlineGame) {
      return
    }

    // If game is waiting for opponent or complete, don't allow moves
    if (gameStatus === 'waiting' || gameStatus === 'complete') {
      return
    }

    // In watch/spectator mode (online but no makeMove function), don't allow interaction
    if (!onlineContext?.makeMove) {
      return
    }

    if (selectedPieceId) {
      const selectedPiece = pieces.find(p => p.id === selectedPieceId)
      if (!selectedPiece) return

      // Check if there's a piece on the target square
      const targetPiece = getPieceAt(file, rank)
      if (targetPiece && targetPiece.isWhite === selectedPiece.isWhite) {
        // Can't move to square occupied by own piece - just deselect
        selectPiece(null)
        return
      }

      // Validate the move
      const board = boardFromFEN(gameState.boardFEN)
      const from = { file: selectedPiece.file, rank: selectedPiece.rank }
      const to = { file, rank }
      const validation = isValidMove(board, from, to)

      if (!validation.valid) {
        // Show invalid move modal
        if (onlineContext?.onInvalidMove && validation.reason && validation.pieceType) {
          onlineContext.onInvalidMove({
            reason: validation.reason,
            pieceType: validation.pieceType
          })
        }
        selectPiece(null)
        return
      }

      // Detect pawn promotion
      if (selectedPiece.type === 'pawn' && (rank === 7 || rank === 0)) {
        const fromNotation = toAlgebraic(from.file, from.rank)
        const toNotation = toAlgebraic(to.file, to.rank)
        if (onlineContext?.onPendingPromotion) {
          onlineContext.onPendingPromotion({
            from: fromNotation,
            to: toNotation,
            pieceId: selectedPieceId,
            isWhite: selectedPiece.isWhite,
            toFile: file,
            toRank: rank,
          })
        }
        selectPiece(null)
        return
      }

      // If online game, optimistically update locally then send to server
      if (isOnlineGame && onlineContext?.makeMove) {
        // Optimistically move the piece immediately for instant feedback
        movePiece(selectedPieceId, file, rank)

        const fromNotation = toAlgebraic(from.file, from.rank)
        const toNotation = toAlgebraic(to.file, to.rank)
        try {
          const response = await onlineContext.makeMove(fromNotation, toNotation)
          // If move was rejected, board will sync back from server
          if (!response.success) {
            console.error('Move rejected by server:', response.error)
          }
        } catch (err) {
          console.error('Failed to make move:', err)
          // Board will sync back from server on error
        }
      } else {
        // Local game - update state directly
        movePiece(selectedPieceId, file, rank)
      }
    } else {
      // Check if there's a piece on this square to select
      const piece = getPieceAt(file, rank)
      if (piece) {
        selectPiece(piece.id)
      }
    }
  }

  const handleSquareHover = (file: number, rank: number) => {
    if (gameState.selectedPieceId) {
      gameState.updateHoverSquare({ file, rank })
    }
  }

  const handleSquareLeave = () => {
    gameState.updateHoverSquare(null)
  }

  return (
    <group ref={squaresRef}>
      {/* Board base */}
      <mesh position={[0, -0.1, 0]} receiveShadow>
        <boxGeometry args={[8.4, 0.2, 8.4]} />
        <meshStandardMaterial color="#2c1810" />
      </mesh>

      {/* Board frame */}
      <mesh position={[0, 0.01, 0]} receiveShadow>
        <boxGeometry args={[8.2, 0.02, 8.2]} />
        <meshStandardMaterial color="#1a0f0a" />
      </mesh>

      {/* Chess squares */}
      {squares.map((square, index) => (
        <BoardSquare
          key={index}
          position={square.position}
          isLight={square.isLight}
          morphProgress={morphProgress}
          isHovered={gameState.hoverSquare?.file === square.file && gameState.hoverSquare?.rank === square.rank}
          hasSelectedPiece={gameState.selectedPieceId !== null}
          onClick={() => handleSquareClick(square.file, square.rank)}
          onHover={() => handleSquareHover(square.file, square.rank)}
          onLeave={handleSquareLeave}
          boardMaterial={boardMaterial}
        />
      ))}
    </group>
  )
}

interface BoardSquareProps {
  position: [number, number, number]
  isLight: boolean
  morphProgress: React.MutableRefObject<{ value: number }>
  isHovered: boolean
  hasSelectedPiece: boolean
  onClick: () => void
  onHover: () => void
  onLeave: () => void
  boardMaterial: BoardMaterial
}

// Create wood texture using canvas (ebony for dark, maple for light)
function createWoodTexture(isLight: boolean): THREE.CanvasTexture {
  const canvas = document.createElement('canvas')
  canvas.width = 256
  canvas.height = 256
  const ctx = canvas.getContext('2d')!

  // Base colors - Maple (light creamy) or Ebony (very dark brown/black)
  let baseR: number, baseG: number, baseB: number
  let grainColor: string
  let grainColor2: string

  if (isLight) {
    // White Maple - creamy white with golden/tan grain
    baseR = 245
    baseG = 232
    baseB = 215
    grainColor = 'rgba(140, 100, 60, 0.55)'    // More visible tan/brown grain
    grainColor2 = 'rgba(170, 130, 90, 0.45)'   // Secondary grain color
  } else {
    // Ebony - medium-brown walnut color with clear grain (definitely not black!)
    baseR = 100
    baseG = 75
    baseB = 60
    grainColor = 'rgba(150, 115, 90, 0.9)'      // Very visible light brown streaks
    grainColor2 = 'rgba(125, 95, 75, 0.85)'     // Clear medium brown streaks
  }

  // Fill base color
  ctx.fillStyle = `rgb(${baseR}, ${baseG}, ${baseB})`
  ctx.fillRect(0, 0, 256, 256)

  // Draw wood grain lines - horizontal with slight waviness
  const grainLines = isLight ? 30 : 25

  for (let i = 0; i < grainLines; i++) {
    ctx.beginPath()
    ctx.strokeStyle = Math.random() > 0.5 ? grainColor : grainColor2
    ctx.lineWidth = Math.random() * 3 + 1  // Thicker lines for more visibility

    const baseY = (i / grainLines) * 256 + (Math.random() - 0.5) * 15
    ctx.moveTo(0, baseY)

    // Create wavy grain line across the width
    for (let x = 0; x <= 256; x += 8) {
      const waveAmplitude = Math.random() * 3 + 1
      const waveFreq = Math.random() * 0.05 + 0.02
      const y = baseY + Math.sin(x * waveFreq + Math.random()) * waveAmplitude
      ctx.lineTo(x, y)
    }
    ctx.stroke()
  }

  // Add some knot-like features (subtle circular patterns)
  const numKnots = Math.floor(Math.random() * 2)
  for (let i = 0; i < numKnots; i++) {
    const knotX = Math.random() * 256
    const knotY = Math.random() * 256
    const knotRadius = Math.random() * 15 + 8

    // Draw concentric ellipses for knot
    for (let r = knotRadius; r > 2; r -= 2) {
      ctx.beginPath()
      ctx.strokeStyle = isLight
        ? `rgba(130, 90, 50, ${0.3 + (knotRadius - r) * 0.03})`   // Darker knots on maple
        : `rgba(90, 65, 45, ${0.45 + (knotRadius - r) * 0.03})`   // Warm brown knots on ebony
      ctx.lineWidth = 1.5
      ctx.ellipse(knotX, knotY, r, r * 0.6, Math.random() * 0.3, 0, Math.PI * 2)
      ctx.stroke()
    }
  }

  // Add fine grain detail lines
  ctx.lineWidth = 1
  for (let i = 0; i < 50; i++) {
    ctx.beginPath()
    ctx.strokeStyle = isLight
      ? `rgba(160, 120, 80, ${Math.random() * 0.25 + 0.15})`   // More visible detail
      : `rgba(95, 70, 50, ${Math.random() * 0.35 + 0.2})`      // Warm brown detail on ebony

    const baseY = Math.random() * 256
    ctx.moveTo(0, baseY)

    for (let x = 0; x <= 256; x += 4) {
      const y = baseY + (Math.random() - 0.5) * 2
      ctx.lineTo(x, y)
    }
    ctx.stroke()
  }

  // Add subtle color variation/noise for natural look
  const imageData = ctx.getImageData(0, 0, 256, 256)
  const data = imageData.data
  for (let i = 0; i < data.length; i += 4) {
    const variation = (Math.random() - 0.5) * 8
    data[i] = Math.max(0, Math.min(255, data[i] + variation))
    data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + variation))
    data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + variation))
  }
  ctx.putImageData(imageData, 0, 0)

  const texture = new THREE.CanvasTexture(canvas)
  texture.wrapS = THREE.RepeatWrapping
  texture.wrapT = THREE.RepeatWrapping
  return texture
}

// Create marble texture using canvas
function createMarbleTexture(isLight: boolean): THREE.CanvasTexture {
  const canvas = document.createElement('canvas')
  canvas.width = 256
  canvas.height = 256
  const ctx = canvas.getContext('2d')!

  // Base color with natural variation
  if (isLight) {
    // Create gradient base for white marble with subtle color shifts
    const gradient = ctx.createRadialGradient(128, 128, 0, 128, 128, 200)
    gradient.addColorStop(0, '#faf8f5')
    gradient.addColorStop(0.5, '#f5f0e8')
    gradient.addColorStop(1, '#ebe5d8')
    ctx.fillStyle = gradient
  } else {
    // Create gradient base for black marble with more visible variation (dark gray, not black)
    const gradient = ctx.createRadialGradient(128, 128, 0, 128, 128, 200)
    gradient.addColorStop(0, '#4a4a4a')
    gradient.addColorStop(0.5, '#3d3d3d')
    gradient.addColorStop(1, '#323232')
    ctx.fillStyle = gradient
  }
  ctx.fillRect(0, 0, 256, 256)

  // Add cloudy background variation for marble
  if (isLight) {
    for (let i = 0; i < 15; i++) {
      const x = Math.random() * 256
      const y = Math.random() * 256
      const radius = Math.random() * 40 + 30
      const cloudGrad = ctx.createRadialGradient(x, y, 0, x, y, radius)
      cloudGrad.addColorStop(0, 'rgba(220, 210, 195, 0.15)')
      cloudGrad.addColorStop(0.5, 'rgba(210, 200, 185, 0.08)')
      cloudGrad.addColorStop(1, 'rgba(200, 190, 175, 0)')
      ctx.fillStyle = cloudGrad
      ctx.fillRect(0, 0, 256, 256)
    }
  } else {
    // Add cloudy variation for black marble (high contrast)
    for (let i = 0; i < 15; i++) {
      const x = Math.random() * 256
      const y = Math.random() * 256
      const radius = Math.random() * 50 + 35
      const cloudGrad = ctx.createRadialGradient(x, y, 0, x, y, radius)
      cloudGrad.addColorStop(0, 'rgba(100, 100, 100, 0.35)')
      cloudGrad.addColorStop(0.5, 'rgba(85, 85, 85, 0.2)')
      cloudGrad.addColorStop(1, 'rgba(60, 60, 60, 0)')
      ctx.fillStyle = cloudGrad
      ctx.fillRect(0, 0, 256, 256)
    }
  }

  // Create marble veins with strong contrast
  const veinColor = isLight ? 'rgba(130, 115, 100, 0.8)' : 'rgba(160, 160, 160, 0.8)'
  const veinColor2 = isLight ? 'rgba(155, 140, 120, 0.6)' : 'rgba(140, 140, 140, 0.65)'
  const veinColor3 = isLight ? 'rgba(120, 105, 90, 0.55)' : 'rgba(130, 130, 130, 0.55)'

  // Draw primary marble veins - bold and visible
  ctx.strokeStyle = veinColor
  ctx.lineWidth = 3
  ctx.lineCap = 'round'

  for (let i = 0; i < 6; i++) {
    ctx.beginPath()
    const startX = Math.random() * 256
    const startY = Math.random() * 256
    ctx.moveTo(startX, startY)

    let x = startX
    let y = startY
    for (let j = 0; j < 6; j++) {
      const cpX = x + (Math.random() - 0.5) * 90
      const cpY = y + (Math.random() - 0.5) * 90
      x = x + (Math.random() - 0.5) * 70
      y = y + (Math.random() - 0.5) * 70
      ctx.quadraticCurveTo(cpX, cpY, x, y)
    }
    ctx.stroke()
  }

  // Add medium veins
  ctx.strokeStyle = veinColor2
  ctx.lineWidth = 2

  for (let i = 0; i < 10; i++) {
    ctx.beginPath()
    const startX = Math.random() * 256
    const startY = Math.random() * 256
    ctx.moveTo(startX, startY)

    let x = startX
    let y = startY
    for (let j = 0; j < 4; j++) {
      const cpX = x + (Math.random() - 0.5) * 60
      const cpY = y + (Math.random() - 0.5) * 60
      x = x + (Math.random() - 0.5) * 50
      y = y + (Math.random() - 0.5) * 50
      ctx.quadraticCurveTo(cpX, cpY, x, y)
    }
    ctx.stroke()
  }

  // Add fine detail veins
  ctx.strokeStyle = veinColor3
  ctx.lineWidth = 1

  for (let i = 0; i < 18; i++) {
    ctx.beginPath()
    const startX = Math.random() * 256
    const startY = Math.random() * 256
    ctx.moveTo(startX, startY)

    let x = startX
    let y = startY
    for (let j = 0; j < 3; j++) {
      const cpX = x + (Math.random() - 0.5) * 40
      const cpY = y + (Math.random() - 0.5) * 40
      x = x + (Math.random() - 0.5) * 35
      y = y + (Math.random() - 0.5) * 35
      ctx.quadraticCurveTo(cpX, cpY, x, y)
    }
    ctx.stroke()
  }

  // Add subtle noise for texture
  const imageData = ctx.getImageData(0, 0, 256, 256)
  const data = imageData.data
  for (let i = 0; i < data.length; i += 4) {
    const noise = (Math.random() - 0.5) * 10
    data[i] = Math.max(0, Math.min(255, data[i] + noise))
    data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + noise))
    data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + noise))
  }
  ctx.putImageData(imageData, 0, 0)

  const texture = new THREE.CanvasTexture(canvas)
  texture.wrapS = THREE.RepeatWrapping
  texture.wrapT = THREE.RepeatWrapping
  return texture
}

// Create resin / epoxy texture — colorful poured resin with swirls and depth
function createResinTexture(isLight: boolean): THREE.CanvasTexture {
  const canvas = document.createElement('canvas')
  canvas.width = 256
  canvas.height = 256
  const ctx = canvas.getContext('2d')!

  if (isLight) {
    // Light resin: milky white / pearl base with subtle pastel swirls
    ctx.fillStyle = '#e8e0d8'
    ctx.fillRect(0, 0, 256, 256)

    // Soft color pools — translucent pastels
    const pools = [
      { color: 'rgba(180, 210, 230, 0.25)', x: 60, y: 80, r: 70 },     // soft blue
      { color: 'rgba(230, 200, 170, 0.3)', x: 180, y: 60, r: 60 },      // warm amber
      { color: 'rgba(200, 230, 200, 0.2)', x: 120, y: 200, r: 55 },     // sage green
      { color: 'rgba(220, 190, 220, 0.2)', x: 200, y: 180, r: 50 },     // lavender
      { color: 'rgba(240, 220, 200, 0.25)', x: 40, y: 180, r: 65 },     // peach
    ]
    for (const p of pools) {
      const g = ctx.createRadialGradient(p.x, p.y, 0, p.x, p.y, p.r)
      g.addColorStop(0, p.color)
      g.addColorStop(1, 'rgba(0,0,0,0)')
      ctx.fillStyle = g
      ctx.fillRect(0, 0, 256, 256)
    }

    // Wispy swirl lines — fluid resin pour marks
    for (let i = 0; i < 8; i++) {
      ctx.beginPath()
      ctx.strokeStyle = `rgba(255, 255, 255, ${0.15 + Math.random() * 0.15})`
      ctx.lineWidth = 1 + Math.random() * 3
      let x = Math.random() * 256, y = Math.random() * 256
      ctx.moveTo(x, y)
      for (let j = 0; j < 5; j++) {
        const cx = x + (Math.random() - 0.5) * 80
        const cy = y + (Math.random() - 0.5) * 80
        x += (Math.random() - 0.5) * 60
        y += (Math.random() - 0.5) * 60
        ctx.quadraticCurveTo(cx, cy, x, y)
      }
      ctx.stroke()
    }
  } else {
    // Dark resin: deep ocean blue / teal base with luminous color swirls
    ctx.fillStyle = '#1a2a3a'
    ctx.fillRect(0, 0, 256, 256)

    // Vivid color pools — like pigment dropped into resin
    const pools = [
      { color: 'rgba(0, 120, 180, 0.35)', x: 70, y: 90, r: 75 },       // deep cyan
      { color: 'rgba(100, 40, 140, 0.3)', x: 190, y: 70, r: 60 },      // purple
      { color: 'rgba(0, 160, 120, 0.25)', x: 130, y: 190, r: 65 },     // emerald
      { color: 'rgba(180, 80, 40, 0.2)', x: 50, y: 190, r: 50 },       // copper
      { color: 'rgba(0, 80, 160, 0.3)', x: 210, y: 180, r: 55 },       // cobalt
    ]
    for (const p of pools) {
      const g = ctx.createRadialGradient(p.x, p.y, 0, p.x, p.y, p.r)
      g.addColorStop(0, p.color)
      g.addColorStop(1, 'rgba(0,0,0,0)')
      ctx.fillStyle = g
      ctx.fillRect(0, 0, 256, 256)
    }

    // Luminous swirl lines — metallic pigment trails
    for (let i = 0; i < 10; i++) {
      ctx.beginPath()
      const colors = ['rgba(100,200,220,0.2)', 'rgba(180,120,200,0.18)', 'rgba(80,180,140,0.15)', 'rgba(200,160,80,0.12)']
      ctx.strokeStyle = colors[i % colors.length]
      ctx.lineWidth = 0.5 + Math.random() * 2.5
      let x = Math.random() * 256, y = Math.random() * 256
      ctx.moveTo(x, y)
      for (let j = 0; j < 6; j++) {
        const cx = x + (Math.random() - 0.5) * 70
        const cy = y + (Math.random() - 0.5) * 70
        x += (Math.random() - 0.5) * 55
        y += (Math.random() - 0.5) * 55
        ctx.quadraticCurveTo(cx, cy, x, y)
      }
      ctx.stroke()
    }
  }

  // Add subtle sparkle/depth noise
  const imageData = ctx.getImageData(0, 0, 256, 256)
  const data = imageData.data
  for (let i = 0; i < data.length; i += 4) {
    const noise = (Math.random() - 0.5) * 6
    data[i] = Math.max(0, Math.min(255, data[i] + noise))
    data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + noise))
    data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + noise))
  }
  ctx.putImageData(imageData, 0, 0)

  const texture = new THREE.CanvasTexture(canvas)
  texture.wrapS = THREE.RepeatWrapping
  texture.wrapT = THREE.RepeatWrapping
  return texture
}

function BoardSquare({
  position,
  isLight,
  morphProgress,
  isHovered,
  hasSelectedPiece,
  onClick,
  onHover,
  onLeave,
  boardMaterial
}: BoardSquareProps) {
  const meshRef = useRef<THREE.Mesh>(null)
  const [localHover, setLocalHover] = useState(false)

  // Colors for 3D and 2D modes (plain)
  const lightColor3D = useMemo(() => new THREE.Color('#e8d4b8'), [])
  const darkColor3D = useMemo(() => new THREE.Color('#8b5a2b'), [])
  const lightColor2D = useMemo(() => new THREE.Color('#f0e6d3'), [])
  const darkColor2D = useMemo(() => new THREE.Color('#5d4037'), [])
  const highlightColor = useMemo(() => new THREE.Color('#7cb342'), [])

  // Marble colors
  const marbleLightColor = useMemo(() => new THREE.Color('#f5f0e8'), [])
  const marbleDarkColor = useMemo(() => new THREE.Color('#404040'), [])  // Dark gray, not black

  // Wood colors - Maple (light) and Ebony (dark)
  const woodLightColor = useMemo(() => new THREE.Color('#f5ebdc'), []) // Maple
  const woodDarkColor = useMemo(() => new THREE.Color('#644b3c'), [])  // Ebony - medium-brown walnut

  // Monochrome - medium matte gray for all squares
  const monochromeColor = useMemo(() => new THREE.Color('#808080'), [])

  // Dayglow Neon colors - hot magenta and electric cyan
  const neonLightColor = useMemo(() => new THREE.Color('#00ffcc'), [])  // Electric cyan/green
  const neonDarkColor = useMemo(() => new THREE.Color('#ff00ff'), [])   // Hot magenta

  // Resin colors — pearlescent light, deep ocean dark
  const resinLightColor = useMemo(() => new THREE.Color('#e8e0d8'), [])
  const resinDarkColor = useMemo(() => new THREE.Color('#1a2a3a'), [])

  // Create marble textures (memoized per square to ensure consistency)
  const marbleTexture = useMemo(() => {
    if (boardMaterial === 'marble') {
      return createMarbleTexture(isLight)
    }
    return null
  }, [boardMaterial, isLight])

  // Create wood textures
  const woodTexture = useMemo(() => {
    if (boardMaterial === 'wood') {
      return createWoodTexture(isLight)
    }
    return null
  }, [boardMaterial, isLight])

  // Create resin textures
  const resinTexture = useMemo(() => {
    if (boardMaterial === 'resin') {
      return createResinTexture(isLight)
    }
    return null
  }, [boardMaterial, isLight])

  useFrame(() => {
    if (meshRef.current) {
      const material = meshRef.current.material as THREE.MeshStandardMaterial
      const progress = morphProgress.current.value

      let targetColor: THREE.Color
      let targetTexture: THREE.CanvasTexture | null = null

      if (boardMaterial === 'neon') {
        // Dayglow Neon: bright saturated colors, no texture
        targetColor = isLight ? neonLightColor.clone() : neonDarkColor.clone()
        targetTexture = null
      } else if (boardMaterial === 'monochrome') {
        // Monochrome: same medium gray for all squares, no texture
        targetColor = monochromeColor.clone()
        targetTexture = null
      } else if (boardMaterial === 'marble') {
        // For marble, use marble colors
        const marbleColor3D = isLight ? marbleLightColor : marbleDarkColor
        const marbleColor2D = isLight ? new THREE.Color('#f8f5f0') : new THREE.Color('#3a3a3a')
        targetColor = marbleColor3D.clone().lerp(marbleColor2D, 1 - progress)
        targetTexture = marbleTexture
      } else if (boardMaterial === 'wood') {
        // For wood, use wood colors (Maple and Ebony)
        const woodColor3D = isLight ? woodLightColor : woodDarkColor
        const woodColor2D = isLight ? new THREE.Color('#f8f0e5') : new THREE.Color('#3a2d24')
        targetColor = woodColor3D.clone().lerp(woodColor2D, 1 - progress)
        targetTexture = woodTexture
      } else if (boardMaterial === 'resin') {
        // Resin / epoxy: glossy poured resin with colorful swirls
        const resinColor3D = isLight ? resinLightColor : resinDarkColor
        const resinColor2D = isLight ? new THREE.Color('#ddd5cd') : new THREE.Color('#162535')
        targetColor = resinColor3D.clone().lerp(resinColor2D, 1 - progress)
        targetTexture = resinTexture
      } else {
        // Plain material - no texture
        targetTexture = null

        // Interpolate colors for plain
        const color3D = isLight ? lightColor3D : darkColor3D
        const color2D = isLight ? lightColor2D : darkColor2D
        targetColor = color3D.clone().lerp(color2D, 1 - progress)
      }

      // Update texture if needed
      if (material.map !== targetTexture) {
        material.map = targetTexture
        material.needsUpdate = true
      }

      // Highlight if hovered and a piece is selected
      if (isHovered && hasSelectedPiece) {
        targetColor = targetColor.clone().lerp(highlightColor, 0.5)
      } else if (localHover && hasSelectedPiece) {
        targetColor = targetColor.clone().lerp(highlightColor, 0.3)
      }

      material.color.copy(targetColor)

      // Adjust material properties based on mode and material type
      if (boardMaterial === 'neon') {
        material.metalness = 0.0
        material.roughness = 0.3   // Slightly glossy plastic
        material.emissive.copy(targetColor)
        material.emissiveIntensity = 0.4 * progress + 0.1  // Glow stronger in 3D
      } else {
        // Reset emissive when switching away from neon
        material.emissive.setHex(0x000000)
        material.emissiveIntensity = 0

        if (boardMaterial === 'monochrome') {
          material.metalness = 0.0
          material.roughness = 0.95  // Very matte cardboard
        } else if (boardMaterial === 'marble') {
          material.metalness = 0.02 * progress
          material.roughness = 0.55 - 0.1 * progress // Marble - matte polished, not glossy
        } else if (boardMaterial === 'wood') {
          material.metalness = 0.0 // Wood should have no metalness at all
          material.roughness = 0.85 + 0.05 * progress // Wood is matte, not shiny (0.85-0.9)
        } else if (boardMaterial === 'resin') {
          material.metalness = 0.05 * progress  // Slight reflectivity for glossy epoxy
          material.roughness = 0.15 + 0.1 * (1 - progress) // Very glossy poured resin
        } else {
          material.metalness = 0.1 * progress
          material.roughness = 0.8 - 0.3 * progress
        }
      }
    }
  })

  const handlePointerOver = (e: ThreeEvent<PointerEvent>) => {
    e.stopPropagation()
    setLocalHover(true)
    onHover()
  }

  const handlePointerOut = (e: ThreeEvent<PointerEvent>) => {
    e.stopPropagation()
    setLocalHover(false)
    onLeave()
  }

  const handleClick = (e: ThreeEvent<MouseEvent>) => {
    e.stopPropagation()
    onClick()
  }

  return (
    <mesh
      ref={meshRef}
      position={position}
      receiveShadow
      onClick={handleClick}
      onPointerOver={handlePointerOver}
      onPointerOut={handlePointerOut}
    >
      <boxGeometry args={[0.98, 0.05, 0.98]} />
      <meshStandardMaterial
        color={isLight ? '#e8d4b8' : '#8b5a2b'}
        metalness={0.1}
        roughness={0.5}
      />
    </mesh>
  )
}
