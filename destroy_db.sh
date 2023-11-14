#!/bin/bash

# set -x

work_dir=$(dirname "$(readlink --canonicalize-existing "${0}" 2> /dev/null)")
readonly docker_compose_file="${work_dir}/docker-compose.yml"
readonly error_docker_file_not_found=80
readonly error_database_data_directory_not_found=81
readonly database_data_dir="${work_dir}/database-data"

confirm() {
	local -r prompt="${1}"
    read -rp "${prompt}" choice
    [[ "$choice" =~ ^[Yy]$ ]]
}

if [[ ! -d "${database_data_dir}" ]]; then
	echo "error: ${database_data_dir} not found" >&2
	exit ${error_database_data_directory_not_found}
fi

if confirm "Are you sure? "; then
    if confirm "Are you REALLY sure? "; then
    	rm -rvf "${database_data_dir}"
    	echo "Done ..."
    fi
fi

exit 0