import { createContext, useContext, useEffect, useState } from "react";
import { sendEvent, onEvent, offEvent } from '../socket';
import { Player } from "../models/player";
import { useNavigate } from "react-router-dom";

type RoleConfig = Record<string, number>;

export interface Narration {
    storyType: string;
    story: string;
    round: number;
}

interface GameContextValue {
    // State
    lobbyId: string | null;
    isHost: boolean;
    players: Player[];
    name: string;
    setName: (n: string) => void;
    role: string | null;
    roleConfig: RoleConfig;
    setRoleConfig: (cfg: RoleConfig) => void;
    theme: string;
    setTheme: (t: string) => void;
    narration: Narration | null;
    // Actions
    createLobby: () => void;
    joinLobby: (code: string) => void;
    startGame: () => void;
}

const GameContext = createContext<GameContextValue | null>(null);

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
            navigate('/game');
        };

        const onStoryNarration = (data: Narration) => {
            if (data.storyType === 'game_intro') setNarration(data);
            else console.log('story-narration (unhandled type):', data.storyType);
        };

        onEvent('connected', onConnected);
        onEvent('lobby-created', onLobbyCreated);
        onEvent('players-updated', onPlayersUpdated);
        onEvent('game-started', onGameStarted);
        onEvent('user-disconnected', onUserDisconnected);
        onEvent('roles-assigned', onRolesAssigned);
        onEvent('story-narration', onStoryNarration);

        return () => {
            offEvent('connected', onConnected);
            offEvent('lobby-created', onLobbyCreated);
            offEvent('players-updated', onPlayersUpdated);
            offEvent('game-started', onGameStarted);
            offEvent('user-disconnected', onUserDisconnected);
            offEvent('roles-assigned', onRolesAssigned);
            offEvent('story-narration', onStoryNarration);
        };
    }, [navigate]);

    const createLobby = () => {
        setNarration(null);
        sendEvent("create-lobby", { name });
    };

    const joinLobby = (code: string) => {
        setNarration(null);
        sendEvent("join-lobby", { lobbyId: code, name });
        setLobbyId(code);
        setIsHost(false);
        navigate('/lobby');
    };

    const startGame = () => {
        console.log("Theme: ", theme);
        sendEvent("start-game", { lobbyId, roles: roleConfig, theme });
    };

    return (
        <GameContext.Provider value={{ lobbyId, isHost, players, name, setName, role, roleConfig, setRoleConfig, theme, setTheme, narration, createLobby, joinLobby, startGame }}>
            {children}
        </GameContext.Provider>
    );
}

export function useGame() {
    const ctx = useContext(GameContext);
    if (!ctx) throw new Error('useGame must be used inside GameProvider');
    return ctx;
}