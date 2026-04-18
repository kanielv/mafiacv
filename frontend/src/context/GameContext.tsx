import { createContext, useContext, useEffect, useRef, useState } from "react";
import { sendEvent, onEvent, offEvent } from '../socket';
import { Player } from "../models/player";
import { useNavigate } from "react-router-dom";

type RoleConfig = Record<string, number>;

export interface Narration {
    storyType: string;
    story: string;
    round: number;
}

export interface SheriffResult {
    targetId: string;
    role: string;
    round: number;
}

export type Phase = 'lobby' | 'intro' | 'night' | 'ended';

interface GameContextValue {
    lobbyId: string | null;
    isHost: boolean;
    players: Player[];
    name: string;
    setName: (n: string) => void;
    role: string | null;
    roleConfig: RoleConfig;
    setRoleConfig: React.Dispatch<React.SetStateAction<RoleConfig>>;
    theme: string;
    setTheme: (t: string) => void;
    narration: Narration | null;
    phase: Phase;
    round: number;
    nightSubmitted: boolean;
    sheriffResult: SheriffResult | null;
    createLobby: () => void;
    joinLobby: (code: string) => void;
    startGame: () => void;
    submitNightAction: (action: 'kill' | 'investigate' | 'protect', targetId: string) => void;
}

const GameContext = createContext<GameContextValue | null>(null);

const INTRO_BEAT_MS = 3000;

export function GameProvider({ children }: { children: React.ReactNode }) {
    const navigate = useNavigate();

    const [lobbyId, setLobbyId] = useState<string | null>(null);
    const [isHost, setIsHost] = useState<boolean>(false);
    const [players, setPlayers] = useState<Player[]>([]);
    const [name, setName] = useState("");
    const [role, setRole] = useState<string | null>(null);
    const [roleConfig, setRoleConfig] = useState<RoleConfig>({});
    const [theme, setTheme] = useState<string>("");
    const [narration, setNarration] = useState<Narration | null>(null);
    const [phase, setPhase] = useState<Phase>('lobby');
    const [round, setRound] = useState<number>(0);
    const [nightSubmitted, setNightSubmitted] = useState<boolean>(false);
    const [sheriffResult, setSheriffResult] = useState<SheriffResult | null>(null);

    const lobbyIdRef = useRef<string | null>(null);
    const roundRef = useRef<number>(0);
    const introTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    useEffect(() => { lobbyIdRef.current = lobbyId; }, [lobbyId]);
    useEffect(() => { roundRef.current = round; }, [round]);

    useEffect(() => {
        const onConnected = (data: { socketId: string }) => console.log('socket:', data.socketId);
        const onLobbyCreated = (data: { lobbyId: string; players: Player[] }) => {
            setLobbyId(data.lobbyId);
            setIsHost(true);
            setPlayers(data.players);
            navigate('/lobby');
        };

        const onPlayersUpdated = (data: { players: Player[] }) => setPlayers(data.players);
        const onGameStarted = () => console.log('game started');

        const onUserDisconnected = (data: { socketId: string }) =>
            setPlayers(prev => prev.filter(p => p.socketID !== data.socketId));

        const onRolesAssigned = (data: { role: string }) => {
            setRole(data.role);
            setPhase('intro');
            setRound(1);
            navigate('/game');
        };

        const onStoryNarration = (data: Narration) => {
            if (data.storyType !== 'game_intro') {
                console.log('story-narration (unhandled type):', data.storyType);
                return;
            }
            setNarration(data);
            if (introTimerRef.current) clearTimeout(introTimerRef.current);
            introTimerRef.current = setTimeout(() => {
                setPhase('night');
                navigate('/night');
            }, INTRO_BEAT_MS);
        };

        const onSheriffResult = (data: SheriffResult) => setSheriffResult(data);
        const onActionAcknowledged = () => setNightSubmitted(true);

        onEvent('connected', onConnected);
        onEvent('lobby-created', onLobbyCreated);
        onEvent('players-updated', onPlayersUpdated);
        onEvent('game-started', onGameStarted);
        onEvent('user-disconnected', onUserDisconnected);
        onEvent('roles-assigned', onRolesAssigned);
        onEvent('story-narration', onStoryNarration);
        onEvent('sheriff-result', onSheriffResult);
        onEvent('action-acknowledged', onActionAcknowledged);

        return () => {
            offEvent('connected', onConnected);
            offEvent('lobby-created', onLobbyCreated);
            offEvent('players-updated', onPlayersUpdated);
            offEvent('game-started', onGameStarted);
            offEvent('user-disconnected', onUserDisconnected);
            offEvent('roles-assigned', onRolesAssigned);
            offEvent('story-narration', onStoryNarration);
            offEvent('sheriff-result', onSheriffResult);
            offEvent('action-acknowledged', onActionAcknowledged);
            if (introTimerRef.current) clearTimeout(introTimerRef.current);
        };
    }, [navigate]);

    const resetGameState = () => {
        setNarration(null);
        setNightSubmitted(false);
        setSheriffResult(null);
        setPhase('lobby');
        setRound(0);
        setRole(null);
    };

    const createLobby = () => {
        resetGameState();
        sendEvent("create-lobby", { name });
    };

    const joinLobby = (code: string) => {
        resetGameState();
        sendEvent("join-lobby", { lobbyId: code, name });
        setLobbyId(code);
        setIsHost(false);
        navigate('/lobby');
    };

    const startGame = () => {
        console.log("Theme: ", theme);
        sendEvent("start-game", { lobbyId, roles: roleConfig, theme });
    };

    const submitNightAction = (action: 'kill' | 'investigate' | 'protect', targetId: string) => {
        if (!lobbyIdRef.current) return;
        sendEvent("night-action", {
            lobbyId: lobbyIdRef.current,
            action,
            targetId,
            round: roundRef.current,
        });
    };

    return (
        <GameContext.Provider value={{
            lobbyId, isHost, players, name, setName, role,
            roleConfig, setRoleConfig, theme, setTheme, narration,
            phase, round, nightSubmitted, sheriffResult,
            createLobby, joinLobby, startGame, submitNightAction,
        }}>
            {children}
        </GameContext.Provider>
    );
}

export function useGame() {
    const ctx = useContext(GameContext);
    if (!ctx) throw new Error('useGame must be used inside GameProvider');
    return ctx;
}
