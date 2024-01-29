# docker build -t cuda-rust:dev .
# docker run -it -d --runtime nvidia --ipc=host -v $(pwd):/home --name cuda-rust cuda-rust:dev
FROM nvcr.io/nvidia/cuda:12.2.0-devel-ubuntu20.04
RUN apt update && apt install -y \
    curl \
    linux-tools-5.15.0-86-generic \
    && apt clean \
    && rm -rf /var/lib/apt/lists/*
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
CMD ["bash"]