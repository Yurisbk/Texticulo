# Texticulo

Encurtador de URLs com **backend Go** (API + redirects) e **frontend React** (Vite, Tailwind, PT/EN).

## Estrutura

- [`backend/`](backend/) — API REST, JWT, MongoDB, OAuth (Google / Microsoft)
- [`frontend/`](frontend/) — SPA React
- [`fly.toml`](fly.toml) — deploy do backend na [Fly.io](https://fly.io)

## Requisitos

- Go 1.22+
- Node 20+
- MongoDB local ou [MongoDB Atlas](https://www.mongodb.com/atlas) (grátis)

## Git sem chaves e deploy (texticulo.io)

- **Nunca** commite arquivos `.env`; use apenas os arquivos `.env.example` como modelo. O [`.gitignore`](.gitignore) já ignora segredos comuns.
- Antes do primeiro push: `npm run audit:secrets` ou `.\scripts\check-no-secrets.ps1`
- Guia completo (Hostinger DNS, Fly, Vercel, onde colocar cada segredo): **[`docs/DEPLOY-TEXTICULO.md`](docs/DEPLOY-TEXTICULO.md)**

## Desenvolvimento local (início rápido) 🚀

Na raiz do projeto, rode **um único comando** para subir backend + frontend:

```bash
npm install        # primeira vez: instala concurrently
npm run dev        # sobe backend (Go) e frontend (Vite) simultaneamente
```

Acesse: **http://localhost:5173**

O backend roda em `localhost:8080` e o Vite faz proxy automático de `/api`. Configure `MONGODB_URI` se não for o MongoDB local padrão.

### Comandos alternativos

| Comando | Descrição |
|---------|-----------|
| `npm run backend` | Roda só o backend Go |
| `npm run frontend` | Roda só o frontend Vite |
| `npm run install:all` | Instala dependências do frontend |

## Backend (configuração)

Banco **MongoDB** (local ou Atlas). Login via **OAuth** (Google e Microsoft).

Variáveis de ambiente:

| Variável | Descrição |
|----------|-----------|
| `PORT` | Porta HTTP (padrão `8080`) |
| `MONGODB_URI` | Connection string (padrão `mongodb://localhost:27017`) |
| `MONGODB_DB` | Nome do banco (padrão `texticulo`) |
| `JWT_SECRET` | Segredo para assinar JWT (obrigatório em produção) |
| `PUBLIC_API_URL` | URL pública da API — usada nos links curtos e nos redirect URIs do OAuth |
| `FRONTEND_URL` | Origem do front (primeiro valor usado no redirect pós-login); CORS aceita lista separada por vírgulas |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | OAuth Google (Web client) |
| `MICROSOFT_CLIENT_ID` / `MICROSOFT_CLIENT_SECRET` | OAuth Microsoft / Azure AD |

### Produção (build)

Defina `VITE_API_URL` com a URL **HTTPS** do backend (ex.: `https://texticulo-api.fly.dev`), **sem** barra no final:

```bash
set VITE_API_URL=https://seu-backend.fly.dev   # Windows cmd
npm run build
```

Na [Vercel](https://vercel.com), adicione a mesma variável em **Project → Settings → Environment Variables** e defina o **Root Directory** como `frontend` (ou importe só a pasta `frontend`).

## API (resumo)

| Método | Rota | Auth |
|--------|------|------|
| `POST` | `/api/shorten` | Opcional (Bearer) |
| `GET` | `/{code}` | — (redirect + clique) |
| `GET` | `/api/auth/google` | — (redirect OAuth) |
| `GET` | `/api/auth/google/callback` | — (OAuth) |
| `GET` | `/api/auth/microsoft` | — (redirect OAuth) |
| `GET` | `/api/auth/microsoft/callback` | — (OAuth) |
| `GET` | `/api/auth/me` | Sim |
| `GET` | `/api/links` | Sim |
| `GET` | `/api/links/{code}/stats` | Sim |
| `DELETE` | `/api/links/{code}` | Sim |
| `GET` | `/health` | — |

**Limites:**
- Rate limit: até **30** requisições `POST /api/shorten` por IP por minuto
- Links por usuário: máximo **5 links** por conta (usuários autenticados)
- Links anônimos: sem limite (não aparecem no dashboard)

## Deploy barato

### MongoDB Atlas (banco gratuito)

1. Crie conta em [mongodb.com/atlas](https://www.mongodb.com/atlas) (free, sem cartão)
2. Crie um cluster M0 (Free Tier) na região `sa-east-1` (São Paulo)
3. Em **Database Access**, crie um usuário com senha
4. Em **Network Access**, adicione `0.0.0.0/0` (ou o IP do Fly)
5. Copie a connection string: `mongodb+srv://user:pass@cluster.mongodb.net`

### Backend — Fly.io

No diretório do repositório:

```bash
fly launch --no-deploy   # ou ajuste app name em fly.toml
fly secrets set JWT_SECRET="..." MONGODB_URI="mongodb+srv://user:pass@cluster.mongodb.net" MONGODB_DB="texticulo" PUBLIC_API_URL="https://<app>.fly.dev" FRONTEND_URL="https://<seu-projeto>.vercel.app" GOOGLE_CLIENT_ID="..." GOOGLE_CLIENT_SECRET="..." MICROSOFT_CLIENT_ID="..." MICROSOFT_CLIENT_SECRET="..."
fly deploy
```

O build usa o [`Dockerfile`](backend/Dockerfile) (binário estático, imagem distroless).

### Frontend — Vercel

- Conecte o repositório
- **Root Directory:** `frontend`
- Variável: `VITE_API_URL` = URL pública do Fly

### Domínio próprio (`www.texticulo.io` / `api.texticulo.io`)

Passo a passo (DNS na Hostinger, Vercel e Fly): [`docs/DEPLOY-TEXTICULO.md`](docs/DEPLOY-TEXTICULO.md).

Resumo: Vercel (`www`) + Fly (`api`) + registros DNS na Hostinger; segredos só nos painéis Fly/Vercel/Atlas, nunca no Git.

## Licença

MIT (ou ajuste conforme o seu projeto).
