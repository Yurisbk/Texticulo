# Git seguro e deploy (texticulo.io)

Este guia cobre: versionar sem chaves, escolha de infraestrutura, onde colocar segredos e DNS na Hostinger.

## 1. Infraestrutura recomendada (padrão do projeto)

| Camada | Provedor sugerido | Motivo |
|--------|-------------------|--------|
| API Go | **Fly.io** (já há [`fly.toml`](../fly.toml)) | Docker, TLS, secrets nativos |
| Frontend React | **Vercel** | Build Vite, domínio customizado |
| Banco | **MongoDB Atlas** M0 | Grátis, connection string só no backend |
| Domínio `texticulo.io` | **Hostinger** | DNS e registro; a hospedagem compartilhada clássica não roda o binário Go de forma adequada |

**Alternativa:** VPS na Hostinger com Docker — backend + nginx servindo o `dist` do React; segredos em `.env` no servidor (fora do Git) ou variáveis do systemd/docker-compose.

## 2. Onde ficam as chaves (nunca no Git)

| Dado | Onde configurar |
|------|-----------------|
| `JWT_SECRET`, `MONGODB_URI`, `GOOGLE_CLIENT_SECRET` | **Fly.io:** `fly secrets set ...` |
| `GOOGLE_CLIENT_ID` | `fly secrets` (recomendado) ou só no Google Cloud Console |
| `PUBLIC_API_URL`, `FRONTEND_URL` | `fly secrets` (URLs públicas, centralizam comportamento) |
| `VITE_API_URL` | **Vercel:** Project → Settings → Environment Variables (build) |
| Connection string Atlas | Só no backend (Fly); nunca no frontend |

## 3. Antes do primeiro `git push`

1. Rodar o script de auditoria: [`scripts/check-no-secrets.ps1`](../scripts/check-no-secrets.ps1) (PowerShell).
2. Confirmar `git status`: não deve listar `.env` (apenas `.env.example`).
3. Não fazer `git add` de arquivos `.env` nem de chaves exportadas.

## 4. Repositório remoto (GitHub / GitLab / Bitbucket)

1. Crie um repositório **vazio** (sem README gerado, se quiser evitar merge inicial).
2. No projeto:

```bash
git remote add origin https://github.com/SEU_USUARIO/texticulo.git
git branch -M main
git push -u origin main
```

Use SSH ou HTTPS conforme sua preferência; **não** commite tokens de acesso no repositório.

## 5. DNS na Hostinger (domínio na Hostinger)

Apontar registros para onde Vercel e Fly indicarem (cada painel mostra CNAME ou A alvo).

Sugestão de subdomínios:

| Nome | Uso | Tipo típico |
|------|-----|-------------|
| `www` | Site React (Vercel) | CNAME → `cname.vercel-dns.com` (ou valor que a Vercel mostrar) |
| `api` | API Go (Fly) | CNAME → `nome-app.fly.dev` ou instruções do Fly para certificado custom |
| `@` (apex) | Redirecionar para `www` | CNAME flatten (ALIAS) ou A conforme Hostinger/Vercel |

Depois de ativo:

- **Vercel:** adicione `www.texticulo.io` (e apex se quiser) em Domains do projeto.
- **Fly:** `fly certs add api.texticulo.io` e configure o registro DNS que o Fly pedir.

Atualize variáveis:

- `PUBLIC_API_URL=https://api.texticulo.io`
- `FRONTEND_URL=https://www.texticulo.io` (e CORS; pode listar `https://texticulo.io` se usar apex)
- `VITE_API_URL=https://api.texticulo.io`

## 6. OAuth (Google)

No [Google Cloud Console](https://console.cloud.google.com/), credenciais OAuth, inclua o redirect URI **exato** da API em produção:

- `https://api.texticulo.io/api/auth/google/callback`

Você pode começar com URLs `*.fly.dev` e `*.vercel.app` e depois adicionar os domínios finais sem remover os antigos durante a transição.

## 7. Ordem prática

1. Push seguro ao Git (sem `.env`).
2. Atlas: cluster + usuário + connection string.
3. Fly: app, `fly secrets`, `fly deploy`.
4. Vercel: projeto `frontend`, `VITE_API_URL`, deploy.
5. Hostinger: registros DNS `www` e `api`.
6. Certificados TLS nos provedores (Vercel/Fly costumam emitir automaticamente).
7. Ajustar secrets e OAuth para URLs finais.

Nenhuma etapa exige commitar chaves no repositório.
