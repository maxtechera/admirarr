# admirarr: menos paneles, más control sobre tu media stack

Si corrés Plex, Radarr, Sonarr, Prowlarr y un downloader, ya sabés que el problema rara vez es "tener el stack andando". El problema aparece después, cuando algo deja de responder, un import se traba, un mount cambia, el VPN se cae o una librería deja de ver archivos. En ese momento, el costo real no es técnico. Es operativo. Perdés foco, saltás entre interfaces, abrís logs, comparás estados y tratás de reconstruir qué pasó.

Ahí nació **admirarr**. No como otra UI para sumar a la pila, sino como una capa operativa para recuperar contexto rápido. La idea es simple: si el stack ya está distribuido entre varias herramientas, al menos el diagnóstico y la operación cotidiana no deberían estarlo.

En vez de abrir cinco tabs para empezar a entender el problema, quería un punto único de entrada desde terminal. Un lugar desde donde ver estado, correr checks, relanzar pasos críticos y volver a tener claridad.

## El problema no es instalar un homelab, es operarlo bien

La etapa de instalación suele ser la más vistosa. Elegís servicios, armás Docker Compose, conectás volúmenes, configurás indexers y dejás una primera versión funcionando. Esa parte tiene una recompensa clara: en algún momento todo arranca y parece resuelto.

Pero la operación real empieza después.

Con el tiempo, cualquier media stack gana complejidad. Aparecen automatizaciones parciales, rutas que dependen de mounts, servicios que hablan entre sí con configuraciones frágiles, credenciales que quedan repartidas, y contenedores que pueden estar "up" aunque el flujo completo esté roto. Cuando algo falla, la interfaz de cada servicio te muestra una parte del problema, pero rara vez la película completa.

Ese fue el patrón que más me cansó. No faltaban herramientas. Faltaba una forma de **coordinar observación y acción** sin desperdiciar energía en cambiar de contexto todo el tiempo.

## Qué quise construir con admirarr

admirarr es un CLI pensado para operar un stack Plex o Jellyfin + herramientas Arr con menos fricción. No intenta reemplazar Radarr, Sonarr o Prowlarr. Tampoco intenta esconder que cada servicio tiene su propia lógica. Lo que sí intenta es darte una interfaz operativa consistente para tres momentos concretos.

### 1. `admirarr status` para tener una foto real del stack

Cuando algo parece raro, la primera necesidad no es ejecutar cambios. Es ver el estado general sin perder tiempo. `status` junta servicios, librerías, descargas, disco y requests en una sola salida. Esa vista no elimina la necesidad de entrar a cada sistema, pero sí te ayuda a decidir **a dónde mirar primero**.

En la práctica, esto cambia la conversación interna. En vez de pensar "voy a abrir Radarr, Sonarr, qBittorrent y Plex a ver qué encuentro", podés empezar con una foto consolidada. Eso reduce fricción cognitiva, que en operaciones chicas pero frecuentes vale muchísimo.

### 2. `admirarr doctor` para diagnosticar antes de improvisar

Muchas fallas en un homelab no son misteriosas. Son repetitivas. Un contenedor dejó de responder. Un puerto cambió. Un servicio quedó saludable a medias. Un path existe en un lado y no en otro. El VPN está arriba, pero no realmente usable. El downloader sigue activo, pero la cadena completa ya se rompió.

`doctor` existe para convertir esos patrones en checks explícitos. La idea no es sólo detectar que algo anda mal, sino **acortar el tiempo entre síntoma y causa probable**. Eso importa porque en entornos self-hosted el costo más grande no suele ser una caída catastrófica. Suele ser una hora perdida persiguiendo señales sueltas.

Si una herramienta puede concentrar esos checks y devolver resultados accionables, el sistema deja de sentirse como una colección de excepciones y empieza a parecer una operación que se puede dominar.

### 3. `admirarr setup` para reconstruir con menos dolor

Una parte poco glamorosa del self-hosting es que muchas veces no querés "arreglar". Querés rehacer una parte del sistema con menos pasos manuales. Cambiar configuración, reconectar servicios, volver a dejar un entorno consistente.

`setup` está pensado desde esa realidad. No como un wizard decorativo, sino como una forma de converger más rápido hacia una configuración sana. Si ya sabés que el problema no se resuelve con inspección, necesitás un camino más corto para volver a un estado operativo.

## Por qué preferí una interfaz de terminal y no otra pantalla

Me interesaba construir algo que pudiera usar yo, pero también cualquier agente o automatización encima. Un CLI bien diseñado tiene dos ventajas fuertes.

La primera es obvia: es rápido, scriptable y más fácil de integrar. La segunda, menos obvia, es que obliga a pensar mejor la superficie de operación. Si un comando tiene que ser claro para una persona y para una máquina, el diseño mejora. Los verbos se vuelven más explícitos. La salida tiene que ser legible. Los JSON dejan de ser un detalle y pasan a ser parte del producto.

Eso fue central en admirarr. No quería una herramienta que sirviera sólo cuando yo ya sé exactamente qué pasó. Quería una herramienta que pudiera ser **interfaz humana e interfaz de agente al mismo tiempo**.

Por eso el repo también incluye archivos como [`SKILL.md`](/SKILL.md), [`AGENTS.md`](/AGENTS.md) y una referencia rápida en la [página principal](/). La apuesta es que operar infraestructura chica y mediana cada vez va a requerir más colaboración entre criterio humano y ejecución asistida.

## El costo oculto de saltar entre paneles

Hay una trampa común cuando hablamos de herramientas de operación: si cada panel es bueno por separado, parece que el sistema también lo es. Pero no siempre.

En stacks distribuidos, el problema no es sólo el número de interfaces. Es el **cambio de contexto**. Cada cambio de contexto te obliga a recordar qué estabas validando, qué hipótesis seguía viva, qué métrica importaba y qué parte del sistema estabas descartando. Ese costo no aparece en dashboards bonitos, pero se siente cada vez que una falla simple te roba media hora.

En mi caso, el objetivo del lanzamiento no era prometer magia. Era reducir ese costo. Menos paneles no significa menos información. Significa mejor secuencia para llegar a la información correcta.

## Qué tipo de usuario se beneficia más

admirarr no está pensado para alguien que quiere tocar una vez un stack de Plex y olvidarse. Está pensado para la persona que ya vive con ese sistema, lo ajusta, lo observa, lo rompe a veces y necesita volver a ponerlo en pie sin drama innecesario.

Si corrés un homelab en serio, seguramente ya desarrollaste tus propios rituales: revisar descargas, verificar salud, mirar requests, inspeccionar contenedores, entender por qué una película no apareció o por qué una serie quedó en cola. admirarr intenta condensar esos rituales operativos en comandos consistentes.

## Qué no intenta hacer

También me importaba dejar un límite claro. admirarr no intenta convertirse en "la única interfaz" del stack. No quiere reemplazar las UIs específicas donde sí tiene sentido profundizar. Tampoco intenta esconder toda la complejidad real detrás de una capa simplificada que termina rompiéndose cuando el entorno cambia.

El objetivo es otro: darte una entrada operativa mejor. Una forma de empezar bien, diagnosticar más rápido y decidir con menos fricción.

## Lo que aprendí construyéndolo

Construir herramientas para uno mismo tiene un riesgo: podés sobreadaptarlas a tu propio flujo. Pero también tiene una ventaja enorme: si el dolor es real, la vara de utilidad es brutalmente honesta. O te ahorra tiempo o no.

En este caso, lo valioso fue traducir molestias difusas en puntos concretos del producto. No diseñar alrededor de una idea abstracta de "homelab management", sino alrededor de preguntas muy específicas:

- ¿Cómo veo rápido si el problema es general o local?
- ¿Cómo reduzco el tiempo entre síntoma y sospecha razonable?
- ¿Cómo rehago una parte del sistema sin sentir que vuelvo a empezar?
- ¿Cómo hago que esa misma superficie sea útil para agentes?

Cuando esas preguntas guían el diseño, el producto deja de ser una colección de features y empieza a tener una tesis.

## Cómo lo usaría alguien que recién lo descubre

Si llegaste hasta acá y te suena familiar el problema, yo arrancaría por tres pasos simples.

Primero, mirar la [guía de instalación](/) y levantar el binario. Segundo, correr el comando de estado para tener una foto inmediata del entorno. Tercero, probar el flujo de diagnóstico antes de esperar a que todo se rompa. Es mejor conocer la herramienta en calma que descubrirla en medio de una falla.

Después de eso, tiene sentido explorar el [repositorio en GitHub](https://github.com/maxtechera/admirarr?utm_source=blog&utm_medium=owned&utm_campaign=admirarr-launch&utm_content=blog_repo), revisar el enfoque de agentes y entender cómo encaja en tu forma actual de operar el stack.

## La idea más importante detrás del launch

Más allá de admirarr, hay una convicción que me interesa probar: a medida que nuestras herramientas personales y operativas se vuelven más complejas, gana valor todo lo que devuelva claridad. No necesariamente más features. No necesariamente más pantallas. Claridad.

A veces eso viene de abstraer. Otras veces, de ordenar mejor. En este caso, elegí ordenar la operación alrededor de comandos legibles y diagnósticos explícitos.

Si eso hace que un homelab se sienta menos caótico y más gobernable, ya vale la pena.

## FAQ

### ¿admirarr reemplaza las UIs de Plex, Radarr o Sonarr?

No. admirarr funciona como una capa operativa y de diagnóstico. Las UIs siguen siendo útiles para tareas específicas y para inspección profunda.

### ¿sirve solo para Plex?

No. El enfoque es más amplio: stacks Plex o Jellyfin combinados con herramientas Arr y servicios asociados.

### ¿por qué un CLI y no otra dashboard web?

Porque una CLI bien diseñada reduce fricción, se integra mejor con automatizaciones y sirve tanto para humanos como para agentes.

### ¿puedo usarlo aunque mi stack ya esté funcionando?

S�. De hecho, ahí aparece mucho del valor. admirarr no es sólo para instalar. También sirve para observar, diagnosticar y recuperar control cuando el sistema ya está en producción personal.

## CTA

Si te interesa probarlo, podés empezar por la [página de instalación](/) o ir directo al [repo de admirarr en GitHub](https://github.com/maxtechera/admirarr?utm_source=blog&utm_medium=owned&utm_campaign=admirarr-launch&utm_content=blog_repo). Y si ya operás un homelab con dolor real, me interesa mucho más tu feedback operativo que un like.
