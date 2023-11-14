FROM golang:1.21.1

WORKDIR /app

# Copia los archivos del proyecto
COPY . .

# Compila la aplicación Go
RUN go build -o leonlib ./cmd/webapp

# Instalar wait-for-it
ADD https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh /wait-for-it.sh
RUN chmod +x /wait-for-it.sh

# Ejecuta wait-for-it para asegurar que la base de datos esté lista antes de iniciar la aplicación
CMD /wait-for-it.sh leonlib:5432 --timeout=45 -- ./leonlib
