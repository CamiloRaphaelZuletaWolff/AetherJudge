# Despliegue 100 % gratuito de AetherJudge

Guía paso a paso, pensada para alguien que **nunca ha usado** Vercel, Render,
Neon, Oracle Cloud ni GitHub Actions. Cada paso dice qué haces, dónde haces
clic, qué pegas y qué deberías ver.

Al final tendrás:

- El frontend en **https://aether-judge.vercel.app** (tu enlace de portafolio).
- La API (api-gateway) en **Render**, con Redis (Key Value) también en Render.
- La base de datos PostgreSQL en **Neon** (no caduca).
- El **executor** (el que compila y ejecuta el código de los concursantes) en
  una VM gratuita de **Oracle Cloud** (o, como alternativa, Google Cloud),
  protegido con **TLS + token**.
- Imágenes Docker construidas por **GitHub Actions** y publicadas en **GHCR**.

> Convención: todo lo que esté entre `<ÁNGULOS>` lo reemplazas por tu valor
> real. Los comandos para **Windows** están en PowerShell; los de
> **macOS/Linux/VM** en bash.

---

## Índice y orden de ejecución

Las secciones están numeradas por tema, pero **hazlas en este orden**:

| # | Qué | Sección |
| --- | --- | --- |
| 1 | Poner el código en `main` (merge + commit de este trabajo) | [13](#13-limpieza-de-git-antes-de-desplegar) |
| 2 | Crear cuentas | [1](#1-cuentas) |
| 3 | Generar el token del executor | [2](#2-secretos) |
| 4 | Crear la base de datos en Neon | [3](#3-postgresql-en-neon) |
| 5 | Construir las imágenes y hacerlas públicas | [6](#6-imágenes-en-ghcr-github-container-registry) |
| 6 | Crear la VM y levantar el executor (Oracle **o** Google) | [4](#4-executor-en-oracle-cloud-always-free-recomendado) / [5](#5-alternativa-executor-en-google-cloud-e2-micro) |
| 7 | Guardar los secretos de GitHub | [10](#10-secretos-de-github) |
| 8 | Desplegar el backend en Render (Blueprint) | [7](#7-render-blueprint-api-gateway--redis) |
| 9 | Cargar los datos de demo | [8](#8-datos-de-demo-seed-con-github-actions) |
| 10 | Apuntar Vercel a la nueva API | [9](#9-vercel-frontend) |
| 11 | Probar todo | [11](#11-verificación-de-punta-a-punta) |

Si algo falla: [12. Solución de problemas](#12-solución-de-problemas).

---

## 0. Arquitectura y costos

```
                         Navegador (cualquier persona)
                                   │ HTTPS
                                   ▼
             ┌────────────────────────────────────────────┐
             │ Vercel (Hobby)  — Next.js                  │
             │ https://aether-judge.vercel.app            │
             └────────────────────────────────────────────┘
                                   │ HTTPS (REST) + WSS (WebSocket)
                                   │ NEXT_PUBLIC_API_URL
                                   ▼
             ┌────────────────────────────────────────────┐
             │ Render (free, Oregon)                      │
             │  aether-api-gateway  (Docker, :10000)      │
             │   ├─ JUDGE_WORKERS=1 (juzga en proceso)    │
             │   ├──► aether-redis  (Key Value free,      │
             │   │     red privada, sin IP pública)       │
             └───┼────────────────────────────────────────┘
                 │                 │
   PostgreSQL+TLS│                 │ gRPC sobre TLS :443
   (sslmode=     │                 │ authorization: Bearer <EXECUTOR_AUTH_TOKEN>
    require)     ▼                 ▼
  ┌──────────────────────┐  ┌─────────────────────────────────────────────┐
  │ Neon (free)          │  │ VM Oracle Always Free (Ampere A1, arm64)    │
  │ AWS us-west-2        │  │  o Google e2-micro (amd64)                  │
  │ Postgres + citext    │  │                                             │
  │ (fuente de verdad)   │  │  caddy :80/:443  ── Let's Encrypt para      │
  └──────────────────────┘  │   │                 <ip-con-guiones>.sslip.io│
                            │   │ h2c (red interna de Docker)             │
                            │   ▼                                         │
                            │  executor :9090 (solo 127.0.0.1)            │
                            │   │ /var/run/docker.sock                    │
                            │   ▼                                         │
                            │  contenedores sandbox efímeros              │
                            │  arena-sandbox-{cpp,python,go}              │
                            │  (sin red, rootfs de solo lectura, no-root) │
                            └─────────────────────────────────────────────┘
                                   ▲ docker pull
                                   │
             GitHub Actions ──► GHCR (ghcr.io/<owner>/aether-*)
             (builds nativos amd64 + arm64)
```

¿Por qué el executor no está en Render? Porque ejecuta código no confiable
lanzando **contenedores Docker hermanos** (ver `docs/adr/0005`), y eso necesita
un daemon de Docker propio. Render, Vercel y casi cualquier PaaS no lo dan.

### Tabla de costos y límites (todo en US$0)

| Pieza | Servicio / plan | Límites que importan | ¿Cuesta algo? |
| --- | --- | --- | --- |
| Frontend | **Vercel Hobby** | Uso personal/no comercial; un portafolio está bien | No |
| API | **Render Free web service** | 512 MB RAM, 0,1 CPU; **se duerme tras 15 min** sin tráfico y tarda **~1 min** en despertar; **750 horas/mes por workspace**, compartidas con **todos** tus servicios gratuitos (tienes otros 3 backends); si se agotan, Render suspende todos los servicios free del workspace hasta el mes siguiente | No |
| Redis | **Render Key Value Free** | 25 MB, **sin persistencia** (se borra al reiniciar — está bien, ver ADR-0004), **solo 1 Key Value gratis activa por workspace** | No |
| Base de datos | **Neon Free** | 0,5 GB por proyecto; el cómputo se suspende tras ~5 min sin uso (la primera consulta tarda ~1 s); cuota mensual de horas de cómputo (de sobra para una demo). Revisa los números vigentes en neon.com/pricing | No |
| Executor (principal) | **Oracle Cloud Always Free** | Ampere A1: hasta **4 OCPU + 24 GB RAM** en total (3.000 OCPU‑h + 18.000 GB‑h al mes), 200 GB de disco, 10 TB/mes de salida, **IPv4 pública incluida**. Ojo: Oracle puede **reclamar instancias Always Free "ociosas"** (CPU, red y memoria muy bajas durante 7 días) — ver [12](#12-solución-de-problemas) | No |
| Executor (alternativa) | **Google Cloud e2-micro** | 1 VM e2-micro en `us-west1`, `us-central1` o `us-east1`; 30 GB de disco **standard** (el *balanced* que propone la consola **se cobra**); 1 GB/mes de salida. **La IPv4 externa en uso se cobra desde 2024: ~US$0,005/h ≈ US$3,65/mes**. Las cuentas nuevas reciben US$300 de crédito por 90 días, así que los primeros 3 meses no pagas; después **sí** | ~US$3,65/mes tras los créditos |
| Imágenes | **GHCR** | Gratis para paquetes **públicos** | No |
| CI/CD | **GitHub Actions** | Minutos ilimitados en repos **públicos**, incluidos los runners `ubuntu-24.04-arm` | No |
| TLS + DNS | **sslip.io + Let's Encrypt** (vía Caddy) | Sin dominio propio; certificado real y renovación automática | No |

> **Sin keep-alive.** No configures pings periódicos para mantener Render
> despierto: las 750 h/mes se comparten con tus otros 3 backends y un servicio
> despierto 24/7 consume 720 h él solo. El frontend ya muestra un aviso
> ("Despertando el servidor…") mientras la API arranca.

---

## 1. Cuentas

Crea (o revisa) estas cuentas. Usa **"Continue with GitHub"** siempre que se
ofrezca: así todo queda conectado al repositorio.

1. **GitHub** — ya la tienes. Repositorio:
   `https://github.com/CamiloRaphaelZuletaWolff/AetherJudge` (es **público**,
   lo cual es necesario para los runners ARM gratuitos y GHCR gratis).
2. **Render** — <https://render.com> → **Get Started** → **GitHub**. Autoriza a
   Render a leer tus repositorios (puedes limitarlo a `AetherJudge`).
3. **Vercel** — ya existe el proyecto `aether-judge`. Solo comprueba que puedes
   entrar en <https://vercel.com/dashboard>.
4. **Neon** — <https://neon.tech> → **Sign up** → **Continue with GitHub**.
5. **Oracle Cloud** — <https://www.oracle.com/cloud/free/> → **Start for free**.
   Detalles importantes en la [sección 4.1](#41-registro-en-oracle-cloud-lee-esto-antes).
6. *(Solo si usarás la alternativa)* **Google Cloud** —
   <https://console.cloud.google.com> → activa la prueba gratuita.

Herramientas locales (en tu PC):

- **Git** y un terminal (PowerShell en Windows).
- **OpenSSH** (`ssh`, `ssh-keygen`) — Windows 10/11 ya lo trae.
- *(Opcional)* **Go 1.26.4+**, solo si quieres ejecutar el seed localmente en
  lugar de con GitHub Actions.

---

## 2. Secretos

Necesitas **un** secreto inventado por ti: el **token del executor**
(`EXECUTOR_AUTH_TOKEN`). Es la contraseña compartida entre la API (Render) y el
executor (VM). Debe tener **al menos 32 caracteres** y **ningún espacio**.
Usaremos 64 caracteres hexadecimales (sin símbolos raros, a prueba de copiar y
pegar).

**Windows (PowerShell):**

```powershell
$b = New-Object byte[] 32
[Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
-join ($b | ForEach-Object { $_.ToString('x2') })
```

**macOS / Linux / la VM (bash):**

```bash
openssl rand -hex 32
```

Copia el resultado a un gestor de contraseñas o a un archivo de notas privado.
Lo pegarás **en dos sitios** (idéntico, byte a byte):

1. En la VM: `infra/vm/.env` → `EXECUTOR_AUTH_TOKEN=...`
2. En Render: variable `EXECUTOR_AUTH_TOKEN`.

El `JWT_SECRET` de la API **no** lo generas tú: Render lo crea solo
(`generateValue: true` en `render.yaml`).

Además crearás una **clave SSH** para entrar a la VM (sección 4.4).

> Nunca subas estos valores al repositorio. `.env` y `infra/vm/.env` están en
> `.gitignore`.

### Valores que irás recopilando

Abre una nota y ve rellenando:

| Nombre | De dónde sale | Ejemplo |
| --- | --- | --- |
| `EXECUTOR_AUTH_TOKEN` | Sección 2 | `9f2c…` (64 hex) |
| `NEON_DATABASE_URL` | Neon, sección 3 | `postgresql://neondb_owner:…@ep-cool-name-a1b2c3d4.us-west-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require` |
| `VM_IP` | Oracle/Google, sección 4/5 | `129.146.10.20` |
| `EXECUTOR_DOMAIN` | `VM_IP` con guiones + `.sslip.io` | `129-146-10-20.sslip.io` |
| `GATEWAY_URL` | Render, sección 7 | `https://aether-api-gateway-xxxx.onrender.com` |
| `FRONTEND_URL` | Vercel | `https://aether-judge.vercel.app` |

---

## 3. PostgreSQL en Neon

### 3.1 Crear el proyecto

1. Entra en <https://console.neon.tech>.
2. **New Project** (o el asistente inicial si es tu primera vez).
3. Rellena:
   - **Project name:** `aether-judge`
   - **Postgres version:** la más reciente que ofrezca (16 o 17; el esquema es
     SQL estándar + la extensión `citext`).
   - **Cloud provider:** **AWS**
   - **Region:** **AWS US West 2 (Oregon)** — `us-west-2`. Es la misma zona
     geográfica que Render Oregon, así la latencia API↔BD es de ~1–2 ms.
   - **Database name:** déjalo en `neondb` (o `aetherjudge`; da igual).
4. **Create project**.

### 3.2 Copiar la cadena de conexión DIRECTA

1. En el panel del proyecto pulsa **Connect** (arriba a la derecha).
2. En la ventana:
   - **Branch:** `main` · **Database:** `neondb` · **Role:** `neondb_owner`.
   - **Connection pooling:** **DESACTÍVALO** (interruptor en *off*).
3. Comprueba que el host **NO** contiene `-pooler`:
   - ✅ Directa: `ep-cool-name-a1b2c3d4.us-west-2.aws.neon.tech`
   - ❌ Pooled: `ep-cool-name-a1b2c3d4-pooler.us-west-2.aws.neon.tech`
4. Copia la cadena completa. Debe terminar en `?sslmode=require` (puede traer
   además `&channel_binding=require`; el driver `pgx` de la API lo soporta).
   Guárdala como `NEON_DATABASE_URL`.

**¿Por qué la directa y no la *pooled*?** Al arrancar, la API aplica las
migraciones con **goose**, que toma un **advisory lock de sesión** de Postgres
para que dos réplicas no migren a la vez. El pooler de Neon (PgBouncer en modo
*transaction*) reparte las sentencias de una misma "sesión" entre conexiones
distintas, así que ese lock no funciona (o se queda colgado). La API abre como
máximo un puñado de conexiones, así que la directa sobra.

### 3.3 citext

El esquema usa `CREATE EXTENSION IF NOT EXISTS citext` (usuarios y correos sin
distinguir mayúsculas). Neon **incluye `citext`** y el rol `neondb_owner` puede
crearla: no tienes que hacer nada. **No crees tablas a mano**: la API ejecuta
las migraciones sola al arrancar, y el seed también (sección 8).

---

## 4. Executor en Oracle Cloud Always Free (recomendado)

El executor necesita una máquina Linux con Docker. Oracle regala una VM ARM
(Ampere A1) muy generosa: 2–4 CPU y 12–24 GB de RAM, más que suficiente para
compilar C++ y Go rápido.

### 4.1 Registro en Oracle Cloud (lee esto antes)

1. <https://www.oracle.com/cloud/free/> → **Start for free**.
2. **País, nombre, correo** → verifica el correo.
3. **Tarjeta de crédito/débito:** se pide **solo para verificar identidad**.
   Puede aparecer un cargo temporal pequeño (≈US$1) que se revierte. Las
   tarjetas prepago/virtuales a veces se rechazan; usa una tarjeta normal.
   Mientras la cuenta sea *Free Tier* **no te cobran nada**.
4. **Home Region (región principal) — decisión PERMANENTE.** Los recursos
   Always Free **solo** se pueden crear en esa región y **no se puede cambiar
   nunca**. Elige una región de EE. UU. Oeste, cerca de Render Oregon:
   - **US West (Phoenix)** `us-phoenix-1` o **US West (San Jose)**
     `us-sanjose-1`.
   - Evita regiones famosas por no tener capacidad A1 (por ejemplo
     `us-ashburn-1` suele estar saturada).
5. Termina el registro. La cuenta puede tardar de minutos a unas horas en
   quedar activa (te llega un correo "Your Oracle Cloud account is fully
   provisioned").

> **"Out of capacity for shape VM.Standard.A1.Flex"** es el problema más común
> de Oracle. Soluciones, en orden:
> 1. Cambia de **Availability Domain** (AD-1, AD-2, AD-3) en el formulario de
>    creación y reintenta.
> 2. Pide menos: **1 OCPU / 6 GB** (puedes ampliarla después, con la VM
>    apagada, en *Edit → Shape*).
> 3. Reintenta en otro momento del día (madrugada en EE. UU. funciona mejor).
> 4. Convierte la cuenta a **Pay As You Go** (*Billing → Upgrade*). Los
>    recursos Always Free **siguen siendo gratis**, tienes prioridad de
>    capacidad y Oracle deja de reclamar instancias ociosas. Riesgo: si creas
>    algo que **no** es Always Free, te lo cobran. Configura un *Budget* con
>    alerta a US$1 (*Billing → Budgets*) si lo haces.

### 4.2 Crear la red (VCN) con el asistente

1. Menú ☰ (arriba a la izquierda) → **Networking** → **Virtual cloud
   networks**.
2. Arriba a la izquierda, en **Compartment**, deja el *root* (tu tenancy).
3. **Actions** → **Start VCN Wizard** (en consolas antiguas: botón **Start VCN
   Wizard**).
4. Elige **Create VCN with Internet Connectivity** → **Start VCN Wizard**.
5. **VCN name:** `aether-vcn`. Deja los CIDR por defecto
   (`10.0.0.0/16`, pública `10.0.0.0/24`, privada `10.0.1.0/24`).
6. **Next** → **Create**. Espera a que todo salga en verde → **View VCN**.

### 4.3 Abrir los puertos 80 y 443 (Security List)

1. Dentro de `aether-vcn` → pestaña/menú **Security** (o **Security Lists**
   en la columna izquierda).
2. Abre **Default Security List for aether-vcn**.
3. **Security rules** / **Ingress Rules** → **Add Ingress Rules**.
4. Primera regla:
   - **Source Type:** CIDR · **Source CIDR:** `0.0.0.0/0`
   - **IP Protocol:** TCP · **Destination Port Range:** `80`
   - **Description:** `ACME HTTP-01 (Let's Encrypt)`
5. **+ Another Ingress Rule**:
   - **Source CIDR:** `0.0.0.0/0` · **IP Protocol:** TCP ·
     **Destination Port Range:** `443` · **Description:** `gRPC TLS (Caddy)`
6. **Add Ingress Rules**.
7. Verifica que ya existe la regla del puerto **22** (SSH) desde `0.0.0.0/0`;
   el asistente la crea. **No** abras el 9090: el executor escucha solo en
   `127.0.0.1`.

**¿Restringir el 443 a las IP de Render?** Es opcional:

- En Render: servicio → **Connect** (arriba a la derecha) → pestaña
  **Outbound** muestra los rangos IP de salida de tu servicio.
- Podrías cambiar la fuente de la regla 443 a esos rangos. **El 80 debe seguir
  abierto a `0.0.0.0/0`** para que Let's Encrypt valide y renueve el
  certificado (Caddy usa el reto HTTP-01 por el puerto 80).
- *Trade-off:* esas IP las comparten **muchos** clientes de Render de la misma
  región, así que el filtro aporta poco, y si Render las cambia tu executor
  deja de responder sin aviso. La protección real es el **token** (sin él toda
  llamada recibe `Unauthenticated`) más TLS. Para una demo, déjalo abierto.

### 4.4 Crear la clave SSH (en tu PC)

**Windows (PowerShell):**

```powershell
ssh-keygen -t rsa -b 4096 -f "$HOME\.ssh\aether_vm" -C "aether-vm"
# Pulsa Enter dos veces (sin passphrase: GitHub Actions usará esta clave)
Get-Content "$HOME\.ssh\aether_vm.pub"
```

**macOS / Linux:**

```bash
ssh-keygen -t rsa -b 4096 -f ~/.ssh/aether_vm -C "aether-vm"
cat ~/.ssh/aether_vm.pub
```

Se crean dos archivos: `aether_vm` (**privada**, no la compartas; irá al
secreto `VM_SSH_KEY` de GitHub) y `aether_vm.pub` (**pública**, se pega en
Oracle). Usamos RSA 4096 porque es compatible con todos los proveedores.

### 4.5 Crear la instancia

1. Menú ☰ → **Compute** → **Instances** → **Create instance**.
2. **Name:** `aether-executor`.
3. **Placement:** deja el **Availability domain** propuesto (si luego da *Out
   of capacity*, vuelve aquí y prueba otro).
4. **Image and shape** → **Edit**:
   - **Change image** → **Ubuntu** → **Canonical Ubuntu 24.04** (cuando elijas
     la *shape* Ampere se usará la variante **aarch64** automáticamente) →
     **Select image**.
   - **Change shape** → **Virtual machine** → **Ampere** →
     **VM.Standard.A1.Flex** → **Number of OCPUs:** `2` · **Amount of memory
     (GB):** `12` → **Select shape**. Debe aparecer la etiqueta
     **"Always Free-eligible"**. (Puedes usar 4 OCPU / 24 GB si es tu única
     instancia A1; 2/12 deja margen para otra VM gratuita.)
5. **Networking** (o *Primary VNIC information*):
   - **Primary network:** *Select existing virtual cloud network* →
     `aether-vcn`.
   - **Subnet:** *Select existing subnet* → **public subnet-aether-vcn**.
   - **Public IPv4 address:** **Assign a public IPv4 address** (✓). Es gratis.
6. **Add SSH keys:** **Paste public keys** → pega el contenido de
   `aether_vm.pub` (una línea que empieza por `ssh-rsa`).
7. **Boot volume:** deja el tamaño por defecto (~47 GB) — entra en los 200 GB
   gratuitos.
8. **Create**. Espera a que el estado pase a **RUNNING** (1–3 min).
9. En la página de la instancia copia **Public IP address** → es tu `VM_IP`.
   Tu `EXECUTOR_DOMAIN` es esa IP con guiones: `129.146.10.20` →
   `129-146-10-20.sslip.io`.

> La IP pública "efímera" de Oracle **se conserva** al reiniciar o detener la
> VM; solo cambia si **terminas** (borras) la instancia.

### 4.6 Entrar por SSH

**Windows:**

```powershell
ssh -i "$HOME\.ssh\aether_vm" ubuntu@<VM_IP>
```

**macOS / Linux:**

```bash
ssh -i ~/.ssh/aether_vm ubuntu@<VM_IP>
```

Escribe `yes` la primera vez. El usuario en las imágenes Ubuntu de Oracle es
**`ubuntu`**. Verás `ubuntu@aether-executor:~$`.

### 4.7 Preparar la VM (`bootstrap.sh`)

En la VM:

```bash
curl -fsSL https://raw.githubusercontent.com/CamiloRaphaelZuletaWolff/AetherJudge/main/infra/vm/bootstrap.sh | bash
```

El script ([`infra/vm/bootstrap.sh`](infra/vm/bootstrap.sh)) es idempotente
(puedes repetirlo) y:

1. Crea **swap** de 4 GB **solo si** la VM tiene < 2 GB de RAM (en A1 no hace
   falta).
2. **Abre 80/443 en el firewall interno (iptables) de Ubuntu de Oracle.** Las
   imágenes de Oracle traen `/etc/iptables/rules.v4` con una cadena `INPUT` que
   solo acepta SSH y termina en `REJECT --reject-with icmp-host-prohibited`.
   Por eso **no basta** con la Security List: el paquete llega a la VM y la
   propia VM lo rechaza. El script inserta reglas `ACCEPT` para 80 y 443 justo
   después de la del 22 y las aplica en caliente. (No usa
   `netfilter-persistent save` a propósito: guardaría también las reglas
   temporales de Docker y tras un reinicio quedarían duplicadas.)
3. Instala **Docker CE** + el plugin **compose** desde el repositorio oficial
   de Docker.
4. Añade tu usuario al grupo `docker`.
5. Clona el repositorio en `~/aether` y crea `~/aether/infra/vm/.env` a partir
   de `.env.example`.

Al terminar **cierra la sesión** (`exit`) y **vuelve a entrar** por SSH para
que el grupo `docker` surta efecto. Comprueba:

```bash
docker ps                 # tabla vacía, SIN "permission denied"
sudo iptables -L INPUT -n --line-numbers | head   # ves ACCEPT para dpt:80 y dpt:443
```

### 4.8 Configurar `infra/vm/.env`

```bash
nano ~/aether/infra/vm/.env
```

Rellena (el resto puede quedarse igual):

```dotenv
EXECUTOR_DOMAIN=129-146-10-20.sslip.io        # tu IP con guiones
EXECUTOR_AUTH_TOKEN=<el token de la sección 2>
EXECUTOR_MAX_CONCURRENT=2                      # 2 con 2 OCPU/12 GB; 4 con 4/24
EXECUTOR_COMPILE_TIMEOUT_MS=20000
GHCR_OWNER=camiloraphaelzuletawolff            # en minúsculas
IMAGE_TAG=latest
```

Guarda con `Ctrl+O`, `Enter`, y sal con `Ctrl+X`. Protégelo:
`chmod 600 ~/aether/infra/vm/.env`.

### 4.9 Desplegar (`deploy.sh`)

> **Antes:** las imágenes deben existir en GHCR y ser públicas
> ([sección 6](#6-imágenes-en-ghcr-github-container-registry)).

```bash
bash ~/aether/infra/vm/deploy.sh
```

[`deploy.sh`](infra/vm/deploy.sh):

1. Valida `.env` (token ≥ 32 caracteres, dominio definido).
2. Detecta el GID del grupo `docker` del host (el executor corre como el
   usuario 10001 y necesita ese grupo para abrir `/var/run/docker.sock`).
3. Si definiste `GHCR_TOKEN`, hace `docker login ghcr.io` (solo para paquetes
   privados).
4. Descarga `aether-executor` y las tres `aether-sandbox-*` (Docker elige la
   arquitectura correcta: arm64 en Oracle, amd64 en Google).
5. Re-etiqueta las sandboxes con los nombres que el executor busca
   localmente: `arena-sandbox-cpp:<tag>`, `arena-sandbox-python:<tag>`,
   `arena-sandbox-go:<tag>` (ver `backend/services/executor/internal/lang`; el
   executor **nunca** las descarga por sí mismo).
6. `docker compose up -d` con dos contenedores ([`docker-compose.yml`](infra/vm/docker-compose.yml)):
   - **executor**: gRPC en `:9090` publicado **solo en `127.0.0.1`**,
     `docker.sock` montado, `restart: unless-stopped`.
   - **caddy**: puertos 80/443, obtiene un certificado de **Let's Encrypt** para
     `EXECUTOR_DOMAIN` y reenvía el gRPC al executor por **h2c** dentro de la
     red de Docker ([`Caddyfile`](infra/vm/Caddyfile)). Cualquier ruta que no
     sea gRPC del executor recibe un 404.
7. Borra imágenes huérfanas y comprueba `https://<EXECUTOR_DOMAIN>/`.

Salida esperada al final:

```
==> OK: TLS is live. On Render set EXECUTOR_ADDR=129-146-10-20.sslip.io:443 and EXECUTOR_TLS=true
...
executor-1  | {"level":"INFO","msg":"starting executor",...,"auth_enabled":true}
executor-1  | {"level":"INFO","msg":"grpc server listening","addr":"[::]:9090"}
```

Si ves `WARN: no valid HTTPS answer yet`, prueba primero **desde tu PC**
`curl -v https://<EXECUTOR_DOMAIN>/` (algunas redes no permiten que la VM se
conecte a su propia IP pública). Si desde tu PC tampoco responde, revisa los puertos
([12](#12-solución-de-problemas)) y `docker compose -f ~/aether/infra/vm/docker-compose.yml logs caddy`.

Comandos útiles del día a día (en `~/aether/infra/vm`):

```bash
docker compose ps                 # ambos "Up"
docker compose logs -f executor   # logs en vivo (Ctrl+C para salir)
docker compose restart executor
bash deploy.sh                    # actualizar a las imágenes más nuevas
bash deploy.sh 3f2a1bc            # desplegar (o volver a) una versión concreta
```

Tras un reinicio de la VM todo vuelve solo (Docker está habilitado y los
contenedores tienen `restart: unless-stopped`).

---

## 5. Alternativa: executor en Google Cloud e2-micro

Úsala solo si Oracle no te da capacidad. Es más lenta (2 vCPU compartidas,
**1 GB de RAM**) y, **tras los créditos de prueba, la IPv4 cuesta ≈US$3,65/mes**.

### 5.1 Proyecto

1. <https://console.cloud.google.com> → selector de proyectos → **New
   Project** → nombre `aether-judge` → **Create**. Anota el **Project ID**
   (p. ej. `aether-judge-451203`).
2. Instala la CLI `gcloud` (<https://cloud.google.com/sdk/docs/install>) y en
   tu PC:

```bash
gcloud auth login
gcloud config set project <PROJECT_ID>
gcloud services enable compute.googleapis.com
```

### 5.2 IP estática (recomendada) o efímera

- **Efímera:** gratis de reservar, pero **cambia si detienes y arrancas** la
  VM (no al reiniciar). Si cambia, tendrías que actualizar `EXECUTOR_DOMAIN`
  y `EXECUTOR_ADDR`.
- **Estática:** misma tarifa mientras está **en uso**; si la VM está detenida
  la IP reservada se cobra más (~US$0,01/h). Recomendada.

```bash
gcloud compute addresses create aether-executor-ip --region=us-west1
gcloud compute addresses describe aether-executor-ip --region=us-west1 --format='value(address)'
```

### 5.3 VM (forma exacta del nivel gratuito)

```bash
gcloud compute instances create aether-executor \
  --zone=us-west1-b \
  --machine-type=e2-micro \
  --image-family=ubuntu-2404-lts-amd64 --image-project=ubuntu-os-cloud \
  --boot-disk-size=30GB --boot-disk-type=pd-standard \
  --address=aether-executor-ip \
  --tags=aether-executor
```

- `pd-standard` **es obligatorio**: la consola web propone *Balanced persistent
  disk*, que **no** entra en el nivel gratuito.
- Solo `us-west1`, `us-central1` y `us-east1` son gratuitas. `us-west1`
  (Oregón) está junto a Render Oregon.

### 5.4 Firewall

```bash
gcloud compute firewall-rules create aether-allow-web \
  --network=default --direction=INGRESS --action=ALLOW \
  --rules=tcp:80,tcp:443 --target-tags=aether-executor \
  --source-ranges=0.0.0.0/0
```

(El SSH ya lo permite la regla `default-allow-ssh`. Las imágenes Ubuntu de
Google **no** traen el iptables restrictivo de Oracle.)

### 5.5 Clave SSH para GitHub Actions

`gcloud compute ssh` gestiona sus propias claves, pero el workflow de despliegue
necesita una fija. Añade la pública de la [sección 4.4](#44-crear-la-clave-ssh-en-tu-pc):
consola → **Compute Engine → VM instances → aether-executor → Edit →
Security and access → SSH Keys → Add item** → pega `aether_vm.pub` → **Save**.
El usuario será el que aparezca al final de la clave (`aether-vm`) — o
cambia ese comentario por el usuario que quieras **antes** de pegarla. Ese
usuario es tu `VM_USER`.

### 5.6 Preparar y desplegar

```bash
ssh -i ~/.ssh/aether_vm <VM_USER>@<VM_IP>
curl -fsSL https://raw.githubusercontent.com/CamiloRaphaelZuletaWolff/AetherJudge/main/infra/vm/bootstrap.sh | bash
exit   # y vuelve a entrar
```

`bootstrap.sh` detecta 1 GB de RAM y crea **4 GB de swap** (imprescindible: una
compilación pide hasta 512 MB). En `~/aether/infra/vm/.env` usa:

```dotenv
EXECUTOR_DOMAIN=<ip-con-guiones>.sslip.io
EXECUTOR_AUTH_TOKEN=<token>
EXECUTOR_MAX_CONCURRENT=1        # ¡1! dos compilaciones a la vez agotan la RAM
EXECUTOR_COMPILE_TIMEOUT_MS=30000
```

y luego `bash ~/aether/infra/vm/deploy.sh`. Se descargarán las imágenes
**amd64** automáticamente. En Render deja `JUDGE_WORKERS=1`.

---

## 6. Imágenes en GHCR (GitHub Container Registry)

El workflow [`.github/workflows/executor-images.yml`](.github/workflows/executor-images.yml)
construye **cuatro imágenes** para **amd64 y arm64** en runners nativos
(`ubuntu-24.04` y `ubuntu-24.04-arm`), sin emulación — importante para la
sandbox de Go, que pre-calienta su `GOCACHE` compilando la librería estándar
para su arquitectura — y las une en manifiestos multi-arquitectura:

```
ghcr.io/camiloraphaelzuletawolff/aether-executor        :latest  :<sha corto>
ghcr.io/camiloraphaelzuletawolff/aether-sandbox-cpp     :latest  :<sha corto>
ghcr.io/camiloraphaelzuletawolff/aether-sandbox-python  :latest  :<sha corto>
ghcr.io/camiloraphaelzuletawolff/aether-sandbox-go      :latest  :<sha corto>
```

(También quedan etiquetas `<sha>-amd64` / `<sha>-arm64`; son las piezas del
manifiesto, puedes ignorarlas.)

### 6.1 Ejecutarlo

Se ejecuta solo en cada push a `main` que toque `backend/services/executor/**`
o `backend/pkg/**`. La primera vez lánzalo a mano:

1. GitHub → repo → pestaña **Actions**.
2. Columna izquierda → **Executor images** → **Run workflow** → Branch `main`
   → **Run workflow**.
3. Espera a que los 8 jobs *Build* y los 4 *Publish* salgan en verde (~10–20
   min la primera vez; luego la caché lo acelera).

> Si "Executor images" no aparece en la lista, el workflow todavía no está en
> `main` → [sección 13](#13-limpieza-de-git-antes-de-desplegar).

### 6.2 Hacer públicos los paquetes (una vez)

Los paquetes nuevos de GHCR nacen **privados**. Para que la VM los descargue sin
credenciales:

1. Ve a tu perfil de GitHub → pestaña **Packages**
   (`https://github.com/CamiloRaphaelZuletaWolff?tab=packages`).
2. Abre `aether-executor` → a la derecha **Package settings**.
3. Abajo, **Danger Zone** → **Change visibility** → **Public** → escribe el
   nombre del paquete para confirmar → **I understand…**.
4. Repite para `aether-sandbox-cpp`, `aether-sandbox-python` y
   `aether-sandbox-go`.

**Alternativa (paquetes privados):** crea un *Personal access token (classic)*
en **Settings → Developer settings → Personal access tokens → Tokens
(classic) → Generate new token**, con **solo** el scope `read:packages`, y en
`infra/vm/.env` pon `GHCR_USER=<tu-usuario>` y `GHCR_TOKEN=<el token>`.
`deploy.sh` hará `docker login` por ti.

### 6.3 Despliegue automático del executor

[`.github/workflows/deploy-executor.yml`](.github/workflows/deploy-executor.yml)
entra por SSH a la VM, actualiza `~/aether` y ejecuta `deploy.sh`:

- **Automático:** al terminar bien *Executor images* en `main`, despliega
  exactamente esas imágenes (etiqueta = sha corto).
- **Manual:** Actions → **Deploy executor** → **Run workflow** → `image_tag`
  (`latest` o un sha corto para volver atrás).

Necesita los secretos `VM_HOST`, `VM_USER`, `VM_SSH_KEY`
([sección 10](#10-secretos-de-github)). Mientras no existan, el job se salta con
un aviso (no falla).

---

## 7. Render Blueprint (api-gateway + Redis)

### 7.0 Limpia lo antiguo primero

El despliegue anterior (`aether-api-gateway-oyfc`) dejó de responder porque su
Postgres gratuito de Render **caducó a los 30 días**. Antes de crear el nuevo:

1. <https://dashboard.render.com> → revisa la lista de servicios.
2. Borra el **Postgres** antiguo (`aether-db` o similar) si sigue ahí:
   servicio → **Settings** → abajo **Delete Database**.
3. Borra el **Key Value** antiguo (`aether-redis`) si existe. **Render solo
   permite UNA Key Value gratuita activa por workspace**; si queda la vieja (o
   si otro de tus 3 backends usa una), el Blueprint fallará al crear la nueva.
   Si otro proyecto **necesita** su Key Value, crea un **workspace nuevo** para
   AetherJudge (menú de workspace arriba a la izquierda → **New Workspace**,
   plan Hobby): tendrá su propia Key Value gratuita **y sus propias 750 h**.
4. Borra el web service antiguo `aether-api-gateway` (**Settings → Delete Web
   Service**) para que no consuma horas ni choque de nombre.

### 7.1 Crear el Blueprint

1. Dashboard → **+ New** (arriba a la derecha) → **Blueprint**.
2. **Connect a repository** → `CamiloRaphaelZuletaWolff/AetherJudge`. (Si no
   aparece: **Configure account** → da acceso a ese repo en GitHub → vuelve.)
3. **Blueprint Name:** `aether-judge`.
4. **Branch:** `main`. (Render lee `render.yaml` **de esa rama**. Si aún no
   fusionaste tu rama de trabajo, o haces la [sección 13](#13-limpieza-de-git-antes-de-desplegar)
   o eliges aquí `RBAC_UI_Improvement` — pero lo recomendable es `main`.)
5. Render muestra lo que va a crear:
   - `aether-api-gateway` — Web Service, Docker, **Free**, Oregon.
   - `aether-redis` — Key Value, **Free**, Oregon.
6. Rellena las variables marcadas para introducir a mano (`sync: false`):

| Variable | Valor |
| --- | --- |
| `DATABASE_URL` | Tu `NEON_DATABASE_URL` (la **directa**, sin `-pooler`) |
| `EXECUTOR_ADDR` | `<EXECUTOR_DOMAIN>:443` → p. ej. `129-146-10-20.sslip.io:443` |
| `EXECUTOR_AUTH_TOKEN` | El token de la sección 2 (sin espacios ni saltos de línea) |
| `FRONTEND_ORIGIN` | `https://aether-judge.vercel.app` — **sin barra final** |

7. **Deploy Blueprint** (o **Apply**). La primera construcción tarda 3–6 min.

Todo lo demás lo fija `render.yaml`:

| Variable | Valor | Por qué |
| --- | --- | --- |
| `APP_ENV` | `production` | Cookies `Secure` + `SameSite=None` (front y API están en dominios distintos), HSTS, y exige un `JWT_SECRET` fuerte |
| `LOG_FORMAT` | `json` | Logs estructurados |
| `GATEWAY_HTTP_ADDR` | `:10000` | Render enruta al puerto 10000 |
| `JWT_SECRET` | generado por Render | ≥ 32 caracteres; si no, la API no arranca |
| `JUDGE_WORKERS` | `1` | Consumidores de la cola dentro del mismo proceso; cabe en 512 MB y nunca supera `EXECUTOR_MAX_CONCURRENT` |
| `REDIS_ADDR` | `fromService` → `connectionString` de `aether-redis` | URL interna `redis://red-xxxx:6379`; sin contraseña (red privada) |
| `EXECUTOR_TLS` | `true` | Habla TLS con Caddy, validando el certificado de Let's Encrypt |

Sobre el Key Value: `ipAllowList: []` = **ninguna** conexión desde Internet;
solo servicios de tu workspace en Oregon. `maxmemoryPolicy: noeviction` evita
que Redis borre en silencio la cola de juzgamiento al llenarse (de todos modos
la cola se reconstruye desde Postgres al arrancar).

### 7.2 Ver los logs y verificar

1. Abre `aether-api-gateway` → pestaña **Logs**. Deberías ver, en orden:
   líneas `migration applied` (00001_init.sql y 00002_add_role.sql, solo la
   primera vez), `executor client configured … tls=true auth=true` y
   `http server listening addr=:10000`. Estado: **Live**.
2. Copia la URL de arriba (p. ej.
   `https://aether-api-gateway-abcd.onrender.com`) → es tu `GATEWAY_URL`.
   **Puede llevar un sufijo aleatorio**: cópiala exacta del panel.
3. Desde tu PC:

```bash
curl https://<GATEWAY_URL>/healthz    # {"status":"ok"}
curl https://<GATEWAY_URL>/readyz     # OK solo si Postgres Y Redis responden
```

(En PowerShell usa `curl.exe` en lugar de `curl`.) `/healthz` es incondicional;
si `/healthz` responde pero `/readyz` no, el problema es `DATABASE_URL` o Redis.

---

## 8. Datos de demo (seed) con GitHub Actions

El plan gratuito de Render **no tiene Shell**, así que el seed se ejecuta
desde GitHub con [`.github/workflows/seed.yml`](.github/workflows/seed.yml): hace
`go run ./cmd/seed` en `backend/services/api-gateway` con `APP_ENV=dev` (el
seeder se niega a correr en `production`; esto solo afecta a ese proceso
puntual, no a la API desplegada) y `DATABASE_URL` = secreto
`NEON_DATABASE_URL`.

El seeder **aplica las migraciones él mismo** antes de insertar, así que
funciona aunque la API todavía no haya arrancado nunca. Es idempotente
(puedes repetirlo).

1. Asegúrate de tener el secreto `NEON_DATABASE_URL`
   ([sección 10](#10-secretos-de-github)).
2. **Actions** → **Seed demo data** → **Run workflow** → en *confirm* escribe
   `seed` → **Run workflow**.
3. Al terminar (≈1–2 min) los logs muestran los usuarios y el concurso
   creados.

Crea, todos con contraseña `password123`:

| Usuario | Rol | Puede |
| --- | --- | --- |
| `alice` | admin | Todo, incluido `/admin` (roles, concursos, problemas, casos de prueba) |
| `carol` | moderator | Ver todos los envíos de un concurso |
| `bob` | user | Competir |

…y el **Arena Demo Contest** con tres problemas y casos ocultos.

**Alternativa local** (si tienes Go 1.26.4+):

```powershell
# PowerShell
cd backend/services/api-gateway
$env:APP_ENV = "dev"; $env:DATABASE_URL = "<NEON_DATABASE_URL>"
go run ./cmd/seed
```

```bash
# bash
cd backend/services/api-gateway
APP_ENV=dev DATABASE_URL='<NEON_DATABASE_URL>' go run ./cmd/seed
```

---

## 9. Vercel (frontend)

El proyecto `aether-judge` ya existe; solo cambia la URL de la API.

1. <https://vercel.com/dashboard> → proyecto **aether-judge** → **Settings** →
   **Environment Variables**.
2. Busca `NEXT_PUBLIC_API_URL` → **⋯** → **Edit** → valor = tu `GATEWAY_URL`
   **exacta**, p. ej. `https://aether-api-gateway-abcd.onrender.com`
   - con `https://`, **sin barra final**, copiada del panel de Render (el
     sufijo aleatorio como `-oyfc` cambia entre servicios).
   - Entornos: al menos **Production** (marca también Preview si quieres).
   - **Save**.
3. **Redeploy SIN caché** (la variable se "hornea" en el build; sin redeploy no
   cambia nada):
   - Pestaña **Deployments** → el último de **Production** → **⋯** →
     **Redeploy** → **desmarca "Use existing Build Cache"** → **Redeploy**.
4. Comprueba en Render que `FRONTEND_ORIGIN` es **exactamente**
   `https://aether-judge.vercel.app` (servicio → **Environment**). Sin `/`
   final, sin ruta, con `https://`. Si lo cambias, **Save Changes** redepliega
   la API.

Notas:

- El WebSocket se deriva de la misma URL (`https://` → `wss://`), así que la
  API **debe** ser HTTPS (Render ya lo es).
- Los *preview deployments* (`aether-judge-git-…vercel.app`) **no** funcionarán
  contra la API: `FRONTEND_ORIGIN` admite un único origen. Haz la demo desde la
  URL de producción.
- **pnpm:** el proyecto declara `"packageManager": "pnpm@11.5.0"` y Vercel ya
  lo construía bien. Si algún día el build falla con un error de versión de
  pnpm, añade en Vercel la variable `ENABLE_EXPERIMENTAL_COREPACK=1` (hace que
  Vercel use exactamente la versión de `packageManager`) y redepliega. Root
  Directory debe ser `frontend` y Node.js 22.x.

---

## 10. Secretos de GitHub

Repo → **Settings** → **Secrets and variables** → **Actions** → pestaña
**Secrets** → **New repository secret**. Crea:

| Nombre | Valor | Lo usa |
| --- | --- | --- |
| `NEON_DATABASE_URL` | La cadena **directa** de Neon (sección 3.2) | `seed.yml` |
| `VM_HOST` | La IP pública de la VM (`129.146.10.20`) | `deploy-executor.yml` |
| `VM_USER` | `ubuntu` (Oracle) o tu usuario (Google, sección 5.5) | `deploy-executor.yml` |
| `VM_SSH_KEY` | Contenido **completo** de la clave **privada** `aether_vm`, incluidas las líneas `-----BEGIN …-----` y `-----END …-----` | `deploy-executor.yml` |
| `VM_SSH_KNOWN_HOSTS` *(opcional, recomendado)* | Salida de `ssh-keyscan -t ed25519 <VM_IP>` | `deploy-executor.yml` (si falta, confía en la huella la primera vez) |

Para copiar la clave privada:

```powershell
Get-Content "$HOME\.ssh\aether_vm" -Raw | Set-Clipboard   # Windows
```

```bash
cat ~/.ssh/aether_vm      # macOS/Linux: copia toda la salida
```

**No** hace falta ningún secreto para GHCR: `executor-images.yml` usa el
`GITHUB_TOKEN` automático (`permissions: packages: write`).

---

## 11. Verificación de punta a punta

1. **Executor** (en la VM):

   ```bash
   cd ~/aether/infra/vm && docker compose ps     # executor y caddy "Up"
   curl -sI https://<EXECUTOR_DOMAIN>/ | head -1 # HTTP/2 404  (TLS válido)
   ```

2. **API**: `curl.exe https://<GATEWAY_URL>/readyz` → OK.
3. Abre **https://aether-judge.vercel.app**. Si la API dormía, verás abajo el
   aviso **"Despertando el servidor (plan gratuito de Render, ~1 min)…"**; se
   ocultará solo cuando la API responda.
4. **Sign in** → `alice` / `password123`.
5. Dashboard → **Arena Demo Contest** → entra.
6. Envía una solución **en cada lenguaje** (C++, Python, Go) al primer problema.
   Ejemplo, si el problema es sumar dos enteros:

   ```cpp
   #include <bits/stdc++.h>
   int main(){ long long a,b; std::cin>>a>>b; std::cout<<a+b<<"\n"; }
   ```

   ```python
   a, b = map(int, input().split())
   print(a + b)
   ```

   ```go
   package main

   import "fmt"

   func main() {
   	var a, b int64
   	fmt.Scan(&a, &b)
   	fmt.Println(a + b)
   }
   ```

   (Adapta al enunciado real que ves en pantalla.) El veredicto pasa de
   `queued` → `running` → **`accepted`** en pocos segundos.
7. **Tiempo real:** abre una ventana de incógnito, entra como `bob`, abre el
   mismo concurso y envía algo. La **tabla de posiciones** se actualiza en
   **ambas** ventanas sin recargar.
8. **RBAC:** como `alice`, `/admin` muestra usuarios y concursos; como `bob`,
   esa ruta se rechaza.
9. **Logs** (opcional): en la VM, `docker compose logs executor | tail`
   muestra líneas `"msg":"grpc request"` con
   `"method":"/executor.v1.ExecutorService/Execute"` y `"code":"OK"`.

Si un envío recibe veredicto, funciona toda la cadena: Vercel → Render → Neon/
Redis → Caddy → executor → sandbox Docker.

### Cómo se actualiza después

| Cambias… | Pasa solo |
| --- | --- |
| `frontend/**` | Vercel redepliega al hacer push a `main` |
| `backend/services/api-gateway/**`, `backend/pkg/**` | Render redepliega (`autoDeploy: true`); las migraciones nuevas corren al arrancar |
| `backend/services/executor/**`, `backend/pkg/**` | *Executor images* construye y publica → *Deploy executor* lo instala en la VM |
| Una variable en Render | **Save Changes** → redeploy |
| `NEXT_PUBLIC_API_URL` | Hay que **redeplegar** en Vercel (valor de build) |

---

## 12. Solución de problemas

**Envíos en `queued` para siempre / logs de la API con `code = Unauthenticated`**
El token no coincide. Copia de nuevo `EXECUTOR_AUTH_TOKEN` en Render **y** en
`infra/vm/.env` (exactamente igual, sin espacios ni salto de línea final;
la API rechaza al arrancar un token con espacios). Tras cambiar `.env`:
`bash ~/aether/infra/vm/deploy.sh`.

**`Unavailable … authentication handshake failed` / `x509: certificate …` / `tls: …`**
- `EXECUTOR_ADDR` debe ser el **nombre** `…sslip.io:443`, no la IP (el
  certificado es para el nombre).
- `EXECUTOR_TLS=true` en Render.
- Caddy aún no tiene certificado: `docker compose logs caddy` en la VM.
- Prueba desde tu PC: `curl -v https://<EXECUTOR_DOMAIN>/` debe mostrar un
  certificado de Let's Encrypt y un 404.

**Caddy no consigue el certificado (ACME falla, `timeout during connect`,
`connection refused`)**
Let's Encrypt no llega al puerto **80**:
1. Oracle: regla de ingreso TCP 80 desde `0.0.0.0/0` en la Security List
   (4.3) **y** iptables abierto (`sudo iptables -L INPUT -n` debe mostrar
   `ACCEPT … dpt:80` **antes** del `REJECT`; si no, vuelve a ejecutar
   `bootstrap.sh`).
2. Google: regla `aether-allow-web` con el tag `aether-executor` **aplicado a
   la VM**.
3. `EXECUTOR_DOMAIN` coincide con la IP pública **actual** (si cambió la IP,
   cambia el nombre).
4. Si ves `too many certificates already issued for "sslip.io"` es un límite
   de Let's Encrypt compartido; espera unas horas y `docker compose restart
   caddy`.

**Executor: `permission denied … /var/run/docker.sock`**
El contenedor no está en el grupo `docker` correcto. `deploy.sh` calcula
`DOCKER_GID` con `getent group docker`; ejecuta siempre `deploy.sh` (no
`docker compose up` a pelo) o exporta `DOCKER_GID` tú mismo.

**Executor: `No such image: arena-sandbox-cpp:latest`**
Faltan las etiquetas locales. `bash deploy.sh` las crea; comprueba con
`docker images | grep arena-sandbox`. El `IMAGE_TAG` del `.env` y el que usa el
executor (`EXECUTOR_IMAGE_TAG`) deben coincidir — `deploy.sh` lo garantiza.

**`docker pull` → `denied` / `unauthorized`**
Los paquetes de GHCR siguen privados (6.2) o `GHCR_OWNER` no está en
minúsculas.

**Oracle: "Out of capacity for shape VM.Standard.A1.Flex"**
Ver la caja de la [sección 4.1](#41-registro-en-oracle-cloud-lee-esto-antes):
otro Availability Domain, 1 OCPU/6 GB, otra hora, o Pay As You Go.

**Oracle detuvo mi VM ("idle instance reclaimed")**
Oracle puede reclamar instancias Always Free con uso muy bajo durante 7 días
(CPU p95 < 20 %, red y memoria también bajas). Arráncala en **Compute →
Instances → Start**; todo vuelve solo. Para evitarlo de verdad: convierte la
cuenta a Pay As You Go (sigue siendo gratis para recursos Always Free).

**La web tarda ~1 minuto en responder la primera vez**
Normal: Render free se duerme tras 15 min sin tráfico. El aviso
"Despertando el servidor…" lo explica. Si el aviso cambia a "El servidor está
tardando más de lo normal", mira los logs de Render.

**Errores de CORS en la consola del navegador**
`FRONTEND_ORIGIN` no coincide **exactamente** con el origen de Vercel
(`https://aether-judge.vercel.app`, sin `/` final). Corrige y guarda.

**Inicio de sesión que se pierde al recargar / a los 15 min**
La cookie de refresh no viaja entre sitios. `APP_ENV` debe ser `production`
(cookie `SameSite=None; Secure`) y ambas URLs HTTPS. Algunos navegadores con
bloqueo estricto de cookies de terceros (Safari, modo incógnito de Chrome con
bloqueo activado) pueden bloquearla igualmente; prueba en una ventana normal.

**La tabla no se actualiza en vivo (WebSocket rechazado, 403 en `/ws`)**
El WebSocket valida el mismo `FRONTEND_ORIGIN` (compara el *host*). Mismo
arreglo que CORS. Y `NEXT_PUBLIC_API_URL` debe ser `https://…` para que el
front use `wss://`.

**Neon: `extension "citext" is not available` / `permission denied to create
extension`**
Estás conectando con un rol sin permisos o a otra base. Usa el rol
`neondb_owner` que da el panel. `citext` está en la lista de extensiones
soportadas de Neon.

**Neon: la API se queda colgada en "migrating" o falla con `prepared
statement … already exists`**
Estás usando el host **`-pooler`**. Cambia `DATABASE_URL` a la conexión
directa (3.2).

**Neon: `password authentication failed`**
Regeneraste la contraseña en Neon: actualiza `DATABASE_URL` en Render **y** el
secreto `NEON_DATABASE_URL`.

**`/readyz` falla con Redis / `redisx: ping`**
`REDIS_ADDR` debe venir del Blueprint (`connectionString` de `aether-redis`) y
el Key Value debe estar en **Oregon** como la API. Si el Blueprint no pudo
crearlo por el límite de una Key Value gratuita por workspace, ver 7.0.

**Todos mis servicios de Render están suspendidos**
Se agotaron las **750 horas/mes del workspace** (compartidas con tus otros
backends). Se reinician el día 1 del mes siguiente. Para separarlo, usa un
workspace propio para AetherJudge (7.0) y **no** pongas pings de keep-alive.

**Vercel: "No Next.js version detected"**
**Settings → General → Root Directory** = `frontend`.

**GCP: la factura no es US$0**
Revisa: disco `pd-standard` (no *balanced*), región `us-west1/us-central1/
us-east1`, una sola e2-micro, y recuerda que la **IPv4 externa** se cobra
(≈US$3,65/mes) cuando se acaban los créditos.

---

## 13. Limpieza de Git antes de desplegar

Render, el workflow *Executor images* (`on: push: main`) y los botones **Run
workflow** de GitHub usan la rama **`main`**. Los workflows nuevos **no
aparecen** en Actions hasta que están en `main`.

Estado actual del repositorio (revisado):

- La rama `RBAC_UI_Improvement` **ya se fusionó** en `origin/main` (PR #8).
- Tu `main` **local** está desactualizado respecto a `origin/main`.
- Hay **cambios sin commitear** en `RBAC_UI_Improvement` (rediseño de UI,
  `DEPLOY.md`, autenticación del executor, `infra/vm/`, workflows nuevos).
- `graphify-out/`, `*.tfstate` (`infra/terraform/local/terraform.tfstate`
  existe en disco), `.env`, `CLAUDE.md` y `MANUAL_SETUP_GUIDE.md` están en
  `.gitignore` y **no** están versionados (comprobado con `git ls-files`, y
  nunca aparecieron en el historial).

Pasos:

```bash
# 1. Asegúrate de no versionar nada sensible
git status
git ls-files | grep -Ei 'tfstate|graphify-out|(^|/)\.env$'   # no debe imprimir nada

# 2. Commitea el trabajo en la rama
git add -A
git status            # revisa la lista: nada de .env, tfstate ni graphify-out
git commit -m "deploy: free-tier redeploy (Neon, Oracle VM executor, TLS+token, GHCR)"
git push origin RBAC_UI_Improvement

# 3. Abre un Pull Request RBAC_UI_Improvement -> main en GitHub,
#    espera a que el CI pase y fusiónalo (Merge pull request).

# 4. Actualiza tu main local
git checkout main
git pull origin main
```

**Ramas viejas de Dependabot:** hay 26 ramas `dependabot/...` en el remoto.
Lo más limpio es desde GitHub: pestaña **Pull requests** → abre cada PR de
Dependabot → **Close pull request** → **Delete branch** (o fusiona las que
pasen el CI). Desde la terminal:

```bash
git branch -r | grep 'origin/dependabot/' | sed 's#origin/##' \
  | xargs -n1 git push origin --delete
git fetch --prune
```

(Dependabot volverá a abrir las actualizaciones que sigan pendientes; es
normal.)
