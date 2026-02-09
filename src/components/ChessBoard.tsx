import { useMemo, useRef, useState } from 'react'
import { useFrame } from '@react-three/fiber'
import type { ThreeEvent } from '@react-three/fiber'
import * as THREE from 'three'
import type { useGameState, PieceState } from '../hooks/useGameState'
import type { BoardMaterial } from '../hooks/useSettings'
import type { OnlineContext } from './ChessScene'
import { isValidMove, toAlgebraic } from '../utils/chessRules'
import type { Board } from '../utils/chessRules'

interface ChessBoardProps {
  morphProgress: React.MutableRefObject<{ value: number }>
  gameState: ReturnType<typeof useGameState>
  boardMaterial: BoardMaterial
  onlineContext?: OnlineContext
}

// Convert gameState pieces to chess rules board format
function createBoard(pieces: PieceState[], whiteToMove: boolean): Board {
  return {
    pieces: pieces.map(p => ({
      type: p.type,
      isWhite: p.isWhite,
      file: p.file,
      rank: p.rank,
    })),
    whiteToMove,
    castlingRights: {
      whiteKingside: true,
      whiteQueenside: true,
      blackKingside: true,
      blackQueenside: true,
    },
    enPassantSquare: null,
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
    const currentTurn = onlineContext?.currentTurn
    const gameStatus = onlineContext?.gameStatus

    // Don't allow any moves if no game session exists
    if (!isOnlineGame) {
      return
    }

    // If game is waiting for opponent or complete, don't allow moves
    if (gameStatus === 'waiting' || gameStatus === 'complete') {
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
      const whiteToMove = currentTurn ? currentTurn === 'white' : selectedPiece.isWhite
      const board = createBoard(pieces, whiteToMove)
      const from = { file: selectedPiece.file, rank: selectedPiece.rank }
      const to = { file, rank }
      const validation = isValidMove(board, from, to)

      if (!validation.valid) {
        console.log('ChessBoard - Invalid move:', validation.reason, 'pieceType:', validation.pieceType)
        console.log('ChessBoard - onInvalidMove callback exists:', !!onlineContext?.onInvalidMove)
        // Show invalid move modal
        if (onlineContext?.onInvalidMove && validation.reason && validation.pieceType) {
          console.log('ChessBoard - Calling onInvalidMove')
          onlineContext.onInvalidMove({
            reason: validation.reason,
            pieceType: validation.pieceType
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
    // Add cloudy variation for black marble (more visible)
    for (let i = 0; i < 12; i++) {
      const x = Math.random() * 256
      const y = Math.random() * 256
      const radius = Math.random() * 45 + 35
      const cloudGrad = ctx.createRadialGradient(x, y, 0, x, y, radius)
      cloudGrad.addColorStop(0, 'rgba(90, 90, 90, 0.25)')
      cloudGrad.addColorStop(0.5, 'rgba(75, 75, 75, 0.15)')
      cloudGrad.addColorStop(1, 'rgba(60, 60, 60, 0)')
      ctx.fillStyle = cloudGrad
      ctx.fillRect(0, 0, 256, 256)
    }
  }

  // Create marble veins - more prominent (lighter veins for dark marble)
  const veinColor = isLight ? 'rgba(160, 145, 130, 0.6)' : 'rgba(110, 110, 110, 0.7)'
  const veinColor2 = isLight ? 'rgba(185, 170, 155, 0.45)' : 'rgba(130, 130, 130, 0.55)'
  const veinColor3 = isLight ? 'rgba(140, 125, 110, 0.5)' : 'rgba(120, 120, 120, 0.6)'

  // Draw primary marble veins - thicker and more visible
  ctx.strokeStyle = veinColor
  ctx.lineWidth = 2.5
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
  ctx.lineWidth = 1.5

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
  ctx.lineWidth = 0.8

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

  useFrame(() => {
    if (meshRef.current) {
      const material = meshRef.current.material as THREE.MeshStandardMaterial
      const progress = morphProgress.current.value

      let targetColor: THREE.Color
      let targetTexture: THREE.CanvasTexture | null = null

      if (boardMaterial === 'marble') {
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
      if (boardMaterial === 'marble') {
        material.metalness = 0.05 * progress
        material.roughness = 0.3 - 0.1 * progress // Marble is smoother/shinier
      } else if (boardMaterial === 'wood') {
        material.metalness = 0.0 // Wood should have no metalness at all
        material.roughness = 0.85 + 0.05 * progress // Wood is matte, not shiny (0.85-0.9)
      } else {
        material.metalness = 0.1 * progress
        material.roughness = 0.8 - 0.3 * progress
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
