FROM python:3.12-slim

# See detection/Dockerfile's comment -- same unbuffered-stdout issue applies
# to any print() in a non-TTY Docker context.
ENV PYTHONUNBUFFERED=1

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir paho-mqtt numpy

COPY simulator/main.py .

CMD ["python", "main.py"]
